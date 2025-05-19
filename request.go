package goaikit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/avast/retry-go/v4"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

var ErrBadOptions = errors.New("bad options")

// AskConfig holds all configurable parameters for an Ask request.
type AskConfig struct {
	Prompt           string
	Model            string
	Temperature      *float64 // Pointer to distinguish between not set and set to 0.0
	MaxTokens        *int64   // Pointer to distinguish between not set and set to 0
	FrequencyPenalty *float64 // Pointer
	PresencePenalty  *float64 // Pointer
	TopP             *float64 // Pointer
	User             string
	Seed             *int64 // Pointer

	// AskSpecificRequestOptions are openai-go client options specific to this Ask call.
	AskSpecificRequestOptions []option.RequestOption
	Retries                   uint // Number of retries for the request
}

// AskOption is a function that configures AskConfig.
type AskOption func(*AskConfig)

// WithPrompt sets the prompt for the Ask request.
func WithPrompt(prompt string) AskOption {
	return func(ac *AskConfig) { ac.Prompt = prompt }
}

// WithModel sets the model for the Ask request.
func WithModel(model string) AskOption {
	return func(ac *AskConfig) { ac.Model = model }
}

// WithTemperature sets the temperature for the Ask request.
func WithTemperature(temp float64) AskOption {
	return func(ac *AskConfig) { ac.Temperature = &temp }
}

// WithMaxTokens sets the maximum number of tokens for the Ask request.
func WithMaxTokens(tokens int64) AskOption {
	return func(ac *AskConfig) { ac.MaxTokens = &tokens }
}

// WithFrequencyPenalty sets the frequency penalty for the Ask request.
func WithFrequencyPenalty(fp float64) AskOption {
	return func(ac *AskConfig) { ac.FrequencyPenalty = &fp }
}

// WithPresencePenalty sets the presence penalty for the Ask request.
func WithPresencePenalty(pp float64) AskOption {
	return func(ac *AskConfig) { ac.PresencePenalty = &pp }
}

// WithTopP sets the TopP sampling parameter for the Ask request.
func WithTopP(topP float64) AskOption {
	return func(ac *AskConfig) { ac.TopP = &topP }
}

// WithUser sets the user identifier for the Ask request.
func WithUser(user string) AskOption {
	return func(ac *AskConfig) { ac.User = user }
}

// WithSeed sets the seed for the Ask request for deterministic outputs.
func WithSeed(seed int64) AskOption {
	return func(ac *AskConfig) { ac.Seed = &seed }
}

// WithAskSpecificRequestOptions adds openai-go request options specific to this Ask call.
func WithAskSpecificRequestOptions(opts ...option.RequestOption) AskOption {
	return func(ac *AskConfig) {
		ac.AskSpecificRequestOptions = append(ac.AskSpecificRequestOptions, opts...)
	}
}

// WithRetries sets the number of retries for the Ask request.
func WithRetries(retries uint) AskOption {
	return func(ac *AskConfig) { ac.Retries = retries }
}

func Ask[Output any](ctx context.Context, client *Client, askOpts ...AskOption) (*Output, error) {
	var output Output

	cfg := AskConfig{
		Retries:                   3, // Default retries
		AskSpecificRequestOptions: make([]option.RequestOption, 0),
	}
	for _, opt := range askOpts {
		opt(&cfg)
	}

	if cfg.Prompt == "" {
		return nil, ErrBadOptions
	}

	if cfg.Model == "" {
		cfg.Model = client.config.DefaultModel
	}

	// Prepare parameters for the openai-go call
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(cfg.Prompt),
		},
		Model: cfg.Model,
	}

	if cfg.MaxTokens != nil {
		params.MaxTokens = param.NewOpt(*cfg.MaxTokens)
	}
	if cfg.Temperature != nil {
		params.Temperature = param.NewOpt(*cfg.Temperature)
	}
	if cfg.FrequencyPenalty != nil {
		params.FrequencyPenalty = param.NewOpt(*cfg.FrequencyPenalty)
	}
	if cfg.PresencePenalty != nil {
		params.PresencePenalty = param.NewOpt(*cfg.PresencePenalty)
	}
	if cfg.TopP != nil {
		params.TopP = param.NewOpt(*cfg.TopP)
	}
	if cfg.User != "" {
		params.User = param.NewOpt(cfg.User)
	}
	if cfg.Seed != nil {
		params.Seed = param.NewOpt(*cfg.Seed)
	}

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

	var chatCompletion *openai.ChatCompletion
	apiCallFunc := func() error {
		// Pass AskSpecificRequestOptions directly to the New call
		resp, err := client.client.Chat.Completions.New(ctx, params, cfg.AskSpecificRequestOptions...)
		if err != nil {
			return err
		}
		chatCompletion = resp
		return nil
	}

	err := retry.Do(
		apiCallFunc,
		retry.Attempts(cfg.Retries),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			client.logger.Debug("Retrying OpenAI request",
				"attempt", n+1,
				"error", err.Error(),
			)
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("OpenAI API call failed after %d attempts: %w", cfg.Retries, err)
	}

	if len(chatCompletion.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI response contained no choices")
	}

	err = json.Unmarshal([]byte(chatCompletion.Choices[0].Message.Content), &output)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal OpenAI response content: %w", err)
	}

	return &output, nil
}
