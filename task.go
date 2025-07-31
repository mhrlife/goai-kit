package goaikit

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

type TaskConfig struct {
	OverrideModel string
	Instructions  string
	Prompt        string
	MCPServers    []responses.ToolMcpParam
	User          string
}

func DeepResearch[OutFormat any | string](
	ctx context.Context,
	client *Client,
	config TaskConfig,
) (OutFormat, *responses.Response, error) {
	if config.OverrideModel == "" {
		config.OverrideModel = client.config.DefaultModel
	}

	outputFormat := responses.ResponseFormatTextConfigUnionParam{}

	var v OutFormat
	switch any(v).(type) {
	case string:
		outputFormat.OfText = &responses.ResponseFormatTextParam{}
	default:
		// I have no idea but this is the only way OpenAI's deep research supports it!
		b, _ := json.MarshalIndent(MarshalToSchema(v), "", " ")
		config.Prompt += fmt.Sprintf(`
# Output structure
Final answer's output schema must follow this json schema:
"""
%s
"""`, string(b))

		outputFormat.OfJSONObject = &shared.ResponseFormatJSONObjectParam{}
	}

	raw, err := client.client.Responses.New(ctx, responses.ResponseNewParams{
		Background:   param.NewOpt(false),
		Instructions: param.NewOpt(config.Instructions),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: param.NewOpt(config.Prompt),
		},
		User:        param.NewOpt(config.User),
		ServiceTier: responses.ResponseNewParamsServiceTierDefault,
		Model:       config.OverrideModel,
		Tools: lo.Map[responses.ToolMcpParam, responses.ToolUnionParam](
			config.MCPServers,
			func(mcp responses.ToolMcpParam, _ int) responses.ToolUnionParam {
				return responses.ToolUnionParam{
					OfMcp: &mcp,
				}
			},
		),
		Text: responses.ResponseTextConfigParam{
			Format: outputFormat,
		},
	})
	if err != nil {
		return v, nil, errors.Wrap(err, "deep research")
	}

	outputText := raw.OutputText()
	switch any(v).(type) {
	case string:
		return any(outputText).(OutFormat), raw, nil
	default:
		if err := json.Unmarshal([]byte(outputText), &v); err != nil {
			return v, nil, errors.Wrap(err, "output format of deep research")
		}

		return v, raw, nil
	}
}

func NewApprovedMCPServer(name string, url string) responses.ToolMcpParam {
	return responses.ToolMcpParam{
		ServerLabel: name,
		ServerURL:   url,
		RequireApproval: responses.ToolMcpRequireApprovalUnionParam{
			OfMcpToolApprovalSetting: param.NewOpt("never"),
		},
	}

}
