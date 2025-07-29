package goaikit

import (
	"context"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/responses"
)

type TaskConfig struct {
	Instructions string
	Prompt       string
}

func Task[T any](ctx context.Context, client *Client, config TaskConfig) (*responses.Response, error) {
	return client.client.Responses.New(ctx, responses.ResponseNewParams{
		Instructions: param.NewOpt(config.Instructions),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: param.NewOpt(config.Prompt),
		},
		User:        param.Opt[string]{},
		ServiceTier: responses.ResponseNewParamsServiceTierDefault,
		Model:       "o4-mini-deep-research",
		Tools: []responses.ToolUnionParam{
			{
				OfMcp: &responses.ToolMcpParam{
					ServerLabel: "geo-info",
					ServerURL:   "https://5d28f17d65ab.ngrok-free.app/default/sse",
					RequireApproval: responses.ToolMcpRequireApprovalUnionParam{
						OfMcpToolApprovalSetting: param.NewOpt("never"),
					},
				},
			},
		},
	})
}
