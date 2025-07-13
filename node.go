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

type AICallNode[Context any, StructuredOutput any] struct {
	Name            string
	Callback        func(ctx context.Context, arg NodeArg[Context], aiOutput *StructuredOutput) (Context, string, error)
	PromptGenerator func() string
	OtherOptions    []AskOption
}

func NewAICallNode[Context any, StructuredOutput any](node AICallNode[Context, StructuredOutput]) Node[Context] {
	return Node[Context]{
		Name: node.Name,
		Runner: func(ctx context.Context, arg NodeArg[Context]) (Context, string, error) {
			prompt := node.PromptGenerator()

			options := []AskOption{
				WithPrompt(prompt),
			}

			if len(node.OtherOptions) > 0 {
				options = append(options, node.OtherOptions...)
			}

			aiOutput, err := Ask[StructuredOutput](ctx, arg.Client, options...)
			if err != nil {
				return arg.Context, "", err
			}

			return node.Callback(ctx, arg, aiOutput)
		},
	}
}
