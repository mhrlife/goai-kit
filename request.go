package goaikit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

var ErrBadOptions = errors.New("bad options")

func Ask[Output any](ctx context.Context, client *Client, options AskOptions) (*Output, error) {
	var output Output

	if options.Prompt == "" {
		return nil, ErrBadOptions
	}

	if options.Model == "" {
		options.Model = client.config.DefaultModel
	}

	// Prepare parameters for the openai-go call, applying defaults/zero checks
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(options.Prompt),
		},
		Model: options.Model, // Model default applied above
	}
	if options.MaxTokens != 0 {
		params.MaxTokens = param.NewOpt(options.MaxTokens)
	}
	// Temperature still uses ParamIfNotZero as 0.0 is a valid value
	params.Temperature = ParamIfNotZero(options.Temperature)

	schema := InferJSONSchema(output)

	params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
			JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
				Strict: param.NewOpt(true),
				Name:   "json_schema_response",
				Schema: schema,
			},
		},
	}

	// Combine client-level request options with request-specific options
	// Request-specific options provided in AskOptions take precedence.
	allRequestOptions := append(client.config.RequestOptions, options.RequestOptions...)

	response, err := client.client.Chat.Completions.New(ctx, params, allRequestOptions...)

	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON content into the output variable
	err = json.Unmarshal([]byte(response.Choices[0].Message.Content), &output)
	if err != nil {
		// Handle unmarshalling error
		return nil, fmt.Errorf("failed to unmarshal response content: %w", err)
	}

	// Return the unmarshaled output
	return &output, nil
}

type AskOptions struct {
	Prompt         string
	Temperature    float64
	MaxTokens      int64
	RequestOptions []option.RequestOption
	Model          string
}

func ParamIfNotZero[T comparable](t T) param.Opt[T] {
	var zero T
	if t == zero {
		return param.Opt[T]{}
	}

	return param.NewOpt(t)
}
