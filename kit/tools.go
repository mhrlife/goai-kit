package kit

import (
	"reflect"
	"strings"

	"github.com/mhrlife/goai-kit/internal/schema"
)

// AgentToolInfo contains metadata about a tool (renamed to avoid conflict with existing ToolInfo)
type AgentToolInfo struct {
	Name        string
	Description string
}

// ToolExecutor is the interface that all tools must implement
type ToolExecutor interface {
	AgentToolInfo() AgentToolInfo
	Execute(ctx *Context) (any, error)
}

// BaseTool provides default AgentToolInfo implementation
// Embed this in your tool structs to get automatic name generation
type BaseTool struct{}

// AgentToolInfo returns empty AgentToolInfo by default
// Override this method in your tool struct to provide custom name/description
func (b BaseTool) AgentToolInfo() AgentToolInfo {
	return AgentToolInfo{
		Name:        "",
		Description: "",
	}
}

// GetAgentToolInfo extracts AgentToolInfo from a tool, using reflection to generate name if needed
func GetAgentToolInfo(tool ToolExecutor) AgentToolInfo {
	info := tool.AgentToolInfo()

	// If name is empty, generate it from type name using reflection
	if info.Name == "" {
		t := reflect.TypeOf(tool)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		info.Name = typeNameToToolName(t.Name())
	}

	return info
}

// typeNameToToolName converts a Go type name to a tool name
// Examples: MyTool -> my_tool, HTTPClient -> http_client
func typeNameToToolName(typeName string) string {
	if typeName == "" {
		return "unnamed_tool"
	}

	var result strings.Builder
	for i, r := range typeName {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Check if previous char is lowercase or if next char is lowercase
			prevIsLower := i > 0 && typeName[i-1] >= 'a' && typeName[i-1] <= 'z'
			nextIsLower := i < len(typeName)-1 && typeName[i+1] >= 'a' && typeName[i+1] <= 'z'

			if prevIsLower || nextIsLower {
				result.WriteRune('_')
			}
		}
		result.WriteRune(r)
	}

	return strings.ToLower(result.String())
}

// ToolSchema represents tool metadata and parameters
type ToolSchema struct {
	Name        string
	ID          string
	Description string
	JSONSchema  map[string]any
}

// BuildToolSchema creates schema metadata for a tool
func BuildToolSchema(tool ToolExecutor) ToolSchema {
	info := GetAgentToolInfo(tool)
	toolID := strings.ToLower(strings.NewReplacer(" ", "_", "-", "_").Replace(info.Name))

	return ToolSchema{
		Name:        info.Name,
		ID:          toolID,
		Description: info.Description,
		JSONSchema:  schema.MarshalToSchema(tool),
	}
}
