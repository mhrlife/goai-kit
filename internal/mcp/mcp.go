package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mhrlife/goai-kit/internal/kit"
	"gopkg.in/yaml.v3"
)

func NewMCPServer(client *kit.Client, name, version string, tools ...kit.ToolExecutor) (*server.MCPServer, error) {
	s := server.NewMCPServer(
		name,
		version,
		server.WithToolCapabilities(false),
	)

	for _, tool := range tools {
		if err := addGenericToolToMCP(client, s, tool); err != nil {
			schema := kit.BuildToolSchema(tool)
			client.logger.Error("Failed to add tool",
				"tool_name", schema.ID,
				"error", err,
			)

			return nil, err
		}

		schema := BuildToolSchema(tool)
		client.logger.Info("Added MCP tool",
			"server_name", name,
			"tool_name", schema.ID,
			"tool_description", schema.Description,
		)
	}

	return s, nil
}

func addGenericToolToMCP(client *Client, s *server.MCPServer, tool ToolExecutor) error {
	schema := BuildToolSchema(tool)

	schemaJSON, err := json.Marshal(schema.JSONSchema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema for tool %s: %w", schema.ID, err)
	}

	mcpTool := mcp.NewToolWithRawSchema(schema.ID, schema.Description, schemaJSON)

	s.AddTool(
		mcpTool,
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			argsJSON, err := json.Marshal(request.Params.Arguments)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal arguments: %w", err)
			}

			// Create a copy of the tool struct
			toolValue := reflect.ValueOf(tool)
			if toolValue.Kind() == reflect.Ptr {
				toolValue = toolValue.Elem()
			}

			// Create new instance and unmarshal args
			toolCopy := reflect.New(toolValue.Type()).Interface().(ToolExecutor)
			if err := json.Unmarshal(argsJSON, toolCopy); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
			}

			// Execute tool
			ctxWrapper := &Context{
				Context: ctx,
				logger:  client.logger,
			}

			result, err := toolCopy.Execute(ctxWrapper)
			if err != nil {
				return nil, fmt.Errorf("tool execution failed: %w", err)
			}

			stringResult := ""
			switch result.(type) {
			case string:
				stringResult = result.(string)
			default:
				yamlMarshalled, err := yaml.Marshal(result)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal result: %w", err)
				}

				stringResult = string(yamlMarshalled)

			}

			return &mcp.CallToolResult{
				Content:           []mcp.Content{mcp.NewTextContent(stringResult)},
				StructuredContent: result,
			}, nil
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
		mux.Handle("/default/sse", sseServer.SSEHandler())

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

/// -------------------------------------------------
/// -------------------------------------------------
/// -------------------------------------------------

type loggedWriter struct {
	http.ResponseWriter
	status int
	buf    *bytes.Buffer
}

func (lw *loggedWriter) WriteHeader(code int) {
	lw.status = code
	lw.ResponseWriter.WriteHeader(code)
}

func (lw *loggedWriter) Write(p []byte) (int, error) {
	fmt.Println(base64.StdEncoding.EncodeToString(p))
	lw.buf.Write(p)                   // capture
	return lw.ResponseWriter.Write(p) // forward
}

func LogHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lw := &loggedWriter{ResponseWriter: w, buf: &bytes.Buffer{}}
		next.ServeHTTP(lw, r)

		// Dump AFTER the request finishes; remove or move if you need live logs.
		log.Printf("\n---- %s %s -> %d ----\n%s\n",
			r.Method, r.URL.Path, lw.status, lw.buf.String())
	})
}

func (l *loggedWriter) Flush() {
	if f, ok := l.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (l *loggedWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := l.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacker not supported")
}

func (l *loggedWriter) CloseNotify() <-chan bool {
	if c, ok := l.ResponseWriter.(http.CloseNotifier); ok {
		return c.CloseNotify()
	}
	ch := make(chan bool, 1)
	close(ch)
	return ch
}

func (l *loggedWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := l.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}
