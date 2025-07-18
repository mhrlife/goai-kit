package goaikit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/avast/retry-go/v4"
	"github.com/henomis/langfuse-go/model"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
	"strings"
	"sync"
)

// AskOption is a function that configures AskConfig.
type AskOption func(*AskConfig)

var ErrBadOptions = errors.New("bad options")

// AskConfig holds all configurable parameters for an Ask request.
type AskConfig struct {
	Prompt           string
	System           string
	Model            string
	Temperature      *float64 // Pointer to distinguish between not set and set to 0.0
	MaxTokens        *int64   // Pointer to distinguish between not set and set to 0
	ReasoningEffort  *string
	FrequencyPenalty *float64 // Pointer
	PresencePenalty  *float64 // Pointer
	TopP             *float64 // Pointer
	User             string
	Seed             *int64 // Pointer
	ExtraFields      map[string]any
	Files            []File

	Tools map[string]AITool // pass a Tool[ToolArgs any]

	// AskSpecificRequestOptions are openai-go lfClient options specific to this Ask call.
	AskSpecificRequestOptions []option.RequestOption
	Retries                   uint // Number of retries for the request

	// Plugin Inputs
	GenerationName string
}

func Ask[Output any](ctx context.Context, client *Client, askOpts ...AskOption) (*Output, error) {
	return ask[Output](ctx, client, askOpts...)
}

func ask[Output any](ctx context.Context, client *Client, askOpts ...AskOption) (*Output, error) {
	var output Output

	cfg := AskConfig{
		Retries:                   3,
		AskSpecificRequestOptions: make([]option.RequestOption, 0),
	}

	reqContext := &Context{
		Context: ctx,
		config:  &cfg,
		logger:  client.logger,
	}

	for _, opt := range askOpts {
		opt(&cfg)
	}

	if cfg.Model == "" {
		cfg.Model = client.config.DefaultModel
	}

	var messages []openai.ChatCompletionMessageParamUnion
	if cfg.System != "" {
		messages = append(messages, openai.SystemMessage(cfg.System))
	}

	messages = append(messages, openai.UserMessage(cfg.Prompt))

	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    cfg.Model,
	}

	// Apply AskConfig options to the parameters
	applyAskConfig(&cfg, &params)

	switch any(output).(type) {
	case string:
	default:
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
	}

	client.applyBeforeRequestHooks(reqContext, params)

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
aa:
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

	_, _ = client.applyAfterRequestHooks(reqContext, chatCompletion, err)

	if err != nil {
		return nil, fmt.Errorf("OpenAI API call failed after %d attempts: %w", cfg.Retries, err)
	}

	if len(chatCompletion.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI response contained no choices")
	}

	params.Messages = append(params.Messages, chatCompletion.Choices[0].Message.ToParam())

	toolCalls := chatCompletion.Choices[0].Message.ToolCalls

	if len(toolCalls) > 0 {
		client.logger.Debug("got tool calls", "count", len(toolCalls))
		var wg sync.WaitGroup
		var lock sync.Mutex

		for _, call := range toolCalls {
			wg.Add(1)

			go func() {
				defer wg.Done()

				client.logger.Debug("Processing tool call",
					"tool", call.Function.Name,
					"arguments", call.Function.Arguments,
				)

				toolContext := &ToolContext{
					Client:  client,
					Context: reqContext,
				}

				toolContext.WithValue("observation_name", call.Function.Name)

				var result any

				_, _ = WithSpan[any](toolContext.Context, client, &model.Span{
					Input: call.Function.Arguments,
					Name:  fmt.Sprintf("tool_call_%s", call.Function.Name),
				}, func(ctx context.Context) (*any, error) {
					toolContext.Context = ctx
					result, err = cfg.Tools[call.Function.Name].Run(toolContext, call.Function.Arguments)
					return &result, err
				})
				if err != nil {
					client.logger.Error("Tool call failed",
						"tool", call.Function.Name,
						"error", err.Error(),
						"arguments", call.Function.Arguments,
					)

					return
				}

				client.logger.Debug("Tool call result",
					"tool", call.Function.Name,
					"result", fmt.Sprintf("%v", result),
				)

				resultJson, _ := json.Marshal(result)

				lock.Lock()
				defer lock.Unlock()

				params.Messages = append(params.Messages, openai.ToolMessage[string](string(resultJson), call.ID))
			}()
		}

		wg.Wait()

		goto aa
	}

	switch any(output).(type) {
	case string:
		return any(&chatCompletion.Choices[0].Message.Content).(*Output), nil
	}

	err = json.Unmarshal([]byte(chatCompletion.Choices[0].Message.Content), &output)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal OpenAI response content: %w", err)
	}

	return &output, nil
}

func applyAskConfig(cfg *AskConfig, params *openai.ChatCompletionNewParams) {
	if cfg.MaxTokens != nil {
		params.MaxTokens = param.NewOpt(*cfg.MaxTokens)
	}
	if cfg.ReasoningEffort != nil {
		params.ReasoningEffort = shared.ReasoningEffort(*cfg.ReasoningEffort)
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
	if cfg.ExtraFields != nil {
		params.SetExtraFields(cfg.ExtraFields)
	}
	if cfg.Files != nil {
		files := make([]openai.ChatCompletionContentPartUnionParam, 0, len(cfg.Files))
		images := make([]openai.ChatCompletionContentPartUnionParam, 0, len(cfg.Files))

		for _, file := range cfg.Files {
			if strings.Contains(file.DataURI, "/pdf") {
				files = append(files,
					openai.FileContentPart(openai.ChatCompletionContentPartFileFileParam{
						FileData: param.NewOpt(file.DataURI),
						Filename: param.NewOpt(file.Name),
					}),
				)
			}

			if strings.Contains(file.DataURI, "image/") {
				images = append(images,
					openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
						URL: file.DataURI,
						//Detail: "high",
					}),
				)
			}

		}

		if len(files) > 0 {
			params.Messages = append([]openai.ChatCompletionMessageParamUnion{openai.UserMessage(files)}, params.Messages...)
		}

		if len(images) > 0 {
			params.Messages = append([]openai.ChatCompletionMessageParamUnion{openai.UserMessage(images)}, params.Messages...)
		}
	}

	if len(cfg.Tools) > 0 {
		toolParams := make([]openai.ChatCompletionToolParam, 0, len(cfg.Tools))

		for _, tool := range cfg.Tools {
			toolInfo := tool.ToolInfo()

			toolParams = append(toolParams, openai.ChatCompletionToolParam{
				Function: shared.FunctionDefinitionParam{
					Name:        toolInfo.ID,
					Strict:      param.NewOpt(true),
					Description: param.NewOpt(toolInfo.Description),
					Parameters:  toolInfo.JSONSchema,
				},
			})
		}

		params.Tools = toolParams
		params.ParallelToolCalls = param.NewOpt(true)
	}
}
