package goaikit

import "context"

type NodeArg[Context any] struct {
	Context  Context
	Client   *Client
	Metadata map[string]any
}

// The string returned by Runner is the name of the next node to execute.
// An empty string "" means the graph execution should stop.
type Node[Context any] struct {
	Name   string
	Runner func(ctx context.Context, arg NodeArg[Context]) (Context, string, error)
}
