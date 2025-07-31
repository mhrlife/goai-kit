package goaikit

import (
	"context"
	"encoding/json"
	"strings"
)

type ToolContext struct {
	context.Context

	Client *Client
}

func (t *ToolContext) WithValue(key, value any) {
	t.Context = context.WithValue(t.Context, key, value)
}

// === Tool Definition ===

type Tool[ToolArgs any] struct {
	Name        string
	Description string
	Runner      func(ctx *ToolContext, args ToolArgs) (any, error)
}

func (t *Tool[ToolArgs]) ToolID() string {
	return strings.ToLower(strings.NewReplacer(" ", "_", "-", "_").Replace(t.Name))
}

func (t *Tool[ToolArgs]) ToolInfo() ToolInfo {
	var argType ToolArgs

	return ToolInfo{
		ID:          t.ToolID(),
		Name:        t.Name,
		Description: t.Description,
		JSONSchema:  MarshalToSchema(argType),
	}
}

func (t *Tool[ToolArgs]) Run(ctx *ToolContext, argsJson string) (any, error) {
	var args ToolArgs
	if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
		return nil, err
	}

	return t.Runner(ctx, args)
}

type ToolInfo struct {
	ID                       string
	Name                     string
	Description              string
	JSONSchema               map[string]any
	ForceMCPStructuredOutput bool
}

type AITool interface {
	ToolInfo() ToolInfo
	Run(ctx *ToolContext, args string) (any, error)
}
