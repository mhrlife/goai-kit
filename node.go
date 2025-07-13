package goaikit

import (
	"context"
	"github.com/pkg/errors"
)

type NodeArg[Context any] struct {
	Context  Context
	Client   *Client
	Metadata map[string]any
}

type Node[Context any] struct {
	Name   string
	Runner func(ctx context.Context, arg NodeArg[Context]) (Context, string, error)
}

func NewNode[Context any](name string, runner func(ctx context.Context, arg NodeArg[Context]) (Context, string, error)) Node[Context] {
	return Node[Context]{
		Name:   name,
		Runner: runner,
	}
}

type AICallNode[Context any, StructuredOutput any] struct {
	Name            string
	Callback        func(ctx context.Context, arg NodeArg[Context], aiOutput *StructuredOutput) (Context, string, error)
	PromptGenerator func(graphContext Context) (string, error)
	OtherOptions    []AskOption
}

func NewAICallNode[Context any, StructuredOutput any](node AICallNode[Context, StructuredOutput]) Node[Context] {
	return Node[Context]{
		Name: node.Name,
		Runner: func(ctx context.Context, arg NodeArg[Context]) (Context, string, error) {
			prompt, err := node.PromptGenerator(arg.Context)
			if err != nil {
				arg.Client.logger.Error("(ai_node) Failed to generate prompt",
					"node_name", node.Name,
					"error", err,
				)

				return arg.Context, "", errors.Wrap(err, "failed to generate prompt for node "+node.Name)
			}

			options := []AskOption{
				WithPrompt(prompt),
			}

			if len(node.OtherOptions) > 0 {
				options = append(options, node.OtherOptions...)
			}

			aiOutput, err := Ask[StructuredOutput](ctx, arg.Client, options...)
			if err != nil {
				arg.Client.logger.Error("(ai_node) AI call failed",
					"node_name", node.Name,
					"error", err,
				)

				return arg.Context, "", errors.Wrap(err, "failed to call AI for node "+node.Name)
			}

			return node.Callback(ctx, arg, aiOutput)
		},
	}
}
