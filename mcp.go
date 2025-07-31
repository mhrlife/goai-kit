package goaikit

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"log/slog"
	"net/http"
	"strings"
)

// OpenAISearchResult is the exact format used by OpenAI's Deep Research search results.
type OpenAISearchResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
	URL   string `json:"url"`
}

type OpenAISearch struct {
	Description string
	Exec        func(query string) ([]OpenAISearchResult, error)
}

type OpenAIFetch struct {
	Description string
	Exec        func(id string) (*OpenAISearchResult, error)
}

type searchArgs struct {
	Query string `json:"query"`
}

type searchResults struct {
	Results []OpenAISearchResult `json:"results"`
}

type fetchArgs struct {
	ID string `json:"id"`
}

func NewMCPServer(client *Client, name, version string, tools ...AITool) (*server.MCPServer, error) {
	s := server.NewMCPServer(
		name,
		version,
		server.WithToolCapabilities(false),
	)

	for _, tool := range tools {
		if err := addGenericToolToMCP(client, s, tool); err != nil {
			client.logger.Error("Failed to add tool",
				"tool_name", tool.ToolInfo().ID,
				"error", err,
			)

			return nil, err
		}

		client.logger.Info("Added MCP tool",
			"server_name", name,
			"tool_name", tool.ToolInfo().ID,
			"tool_description", tool.ToolInfo().Description,
		)
	}

	return s, nil
}

// NewOpenAIDeepResearchMCPServer creates an MCP server specifically for OpenAI's Deep Research
func NewOpenAIDeepResearchMCPServer(name, version string, search OpenAISearch, fetch OpenAIFetch) (*server.MCPServer, error) {
	s := server.NewMCPServer(
		name,
		version,
		server.WithToolCapabilities(false),
	)

	if err := addOpenAISearchTool(s, search); err != nil {
		return nil, fmt.Errorf("failed to add search tool: %w", err)
	}

	if err := addOpenAIFetchTool(s, fetch); err != nil {
		return nil, fmt.Errorf("failed to add fetch tool: %w", err)
	}

	return s, nil
}

func addGenericToolToMCP(client *Client, s *server.MCPServer, tool AITool) error {
	info := tool.ToolInfo()

	schemaJSON, err := json.Marshal(info.JSONSchema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema for tool %s: %w", info.ID, err)
	}

	mcpTool := mcp.NewToolWithRawSchema(info.ID, info.Description, schemaJSON)

	s.AddTool(
		mcpTool,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			argsJSON, err := json.Marshal(request.Params.Arguments)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal arguments: %w", err)
			}

			toolCtx := &ToolContext{
				Context: ctx,
				Client:  client,
			}

			result, err := tool.Run(toolCtx, string(argsJSON))
			if err != nil {
				return nil, fmt.Errorf("tool execution failed: %w", err)
			}

			resultJSON, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result: %w", err)
			}

			// Check if the tool requires structured output only
			if info.ForceMCPStructuredOutput {
				return &mcp.CallToolResult{
					StructuredContent: result,
				}, nil
			}

			return &mcp.CallToolResult{
				Content:           []mcp.Content{mcp.NewTextContent(string(resultJSON))},
				StructuredContent: result,
			}, nil
		},
	)

	return nil
}

func addOpenAISearchTool(s *server.MCPServer, search OpenAISearch) error {
	searchSchema, err := InferJSONSchema(searchArgs{}).MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to generate search schema: %w", err)
	}

	searchTool := mcp.NewToolWithRawSchema("search", search.Description, searchSchema)

	s.AddTool(
		searchTool,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := request.GetString("query", "")
			if query == "" {
				return nil, fmt.Errorf("query parameter is required")
			}

			results, err := search.Exec(query)
			if err != nil {
				return nil, fmt.Errorf("search execution failed: %w", err)
			}

			response := searchResults{
				Results: results,
			}

			responseJSON, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal search results: %w", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Type: "text",
						Text: string(responseJSON),
					},
				},
				StructuredContent: response,
			}, nil
		},
	)

	return nil
}

func addOpenAIFetchTool(s *server.MCPServer, fetch OpenAIFetch) error {
	fetchSchema, err := InferJSONSchema(fetchArgs{}).MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to generate fetch schema: %w", err)
	}

	fetchTool := mcp.NewToolWithRawSchema("fetch", fetch.Description, fetchSchema)

	s.AddTool(
		fetchTool,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id := request.GetString("id", "")
			if id == "" {
				return nil, fmt.Errorf("id parameter is required")
			}

			result, err := fetch.Exec(id)
			if err != nil {
				return nil, fmt.Errorf("fetch execution failed: %w", err)
			}

			if result == nil {
				return nil, fmt.Errorf("fetch returned nil result")
			}

			resultJSON, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal fetch result: %w", err)
			}

			return mcp.NewToolResultText(string(resultJSON)), nil
		},
	)

	return nil
}

type ServerRoute struct {
	Path   string
	Server *server.MCPServer
}

func StartSSEServerWithRoutes(addr string, routes ...ServerRoute) error {
	if len(routes) == 0 {
		return fmt.Errorf("at least one server route is required")
	}

	mux := http.NewServeMux()
	httpSrv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	for _, route := range routes {
		basePath := route.Path
		if !strings.HasPrefix(basePath, "/") {
			basePath = "/" + basePath
		}

		if strings.HasSuffix(basePath, "/") && len(basePath) > 1 {
			basePath = strings.TrimSuffix(basePath, "/")
		}

		sseServer := server.NewSSEServer(
			route.Server,
			server.WithHTTPServer(httpSrv),
			server.WithStaticBasePath(basePath),
			server.WithSSEEndpoint("/sse"),
			server.WithMessageEndpoint("/message"),
		)

		sseEndpointPath := basePath + "/sse"
		mux.Handle(sseEndpointPath, sseServer.SSEHandler())

		messageEndpointPath := basePath + "/message"
		mux.Handle(messageEndpointPath, sseServer.MessageHandler())

		slog.Info("Registered MCP SSE server",
			"base_path", basePath,
			"sse_endpoint", sseEndpointPath,
			"message_endpoint", messageEndpointPath,
		)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "application/json")

			routes_info := make([]map[string]string, len(routes))
			for i, route := range routes {
				basePath := route.Path
				if !strings.HasPrefix(basePath, "/") {
					basePath = "/" + basePath
				}
				if strings.HasSuffix(basePath, "/") && len(basePath) > 1 {
					basePath = strings.TrimSuffix(basePath, "/")
				}

				routes_info[i] = map[string]string{
					"base_path":        basePath,
					"sse_endpoint":     basePath + "/sse",
					"message_endpoint": basePath + "/message",
				}
			}

			response := map[string]interface{}{
				"message": "MCP Server Hub",
				"count":   len(routes),
				"routes":  routes_info,
			}

			json.NewEncoder(w).Encode(response)
			return
		}

		// If no route matches, return 404
		http.NotFound(w, r)
	})

	slog.Info("Starting MCP server hub",
		"address", addr,
		"routes_count", len(routes),
	)

	return http.ListenAndServe(addr, mux)
}

// StartSSEServer - keep the original function for backward compatibility
func StartSSEServer(mcpServer *server.MCPServer, addr string) error {
	slog.Info("Registered one MCP server",
		"addr_for_openai", addr+"/default",
	)

	return StartSSEServerWithRoutes(addr, ServerRoute{
		Path:   "/default",
		Server: mcpServer,
	})
}
