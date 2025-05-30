package goaikit

import (
	"context"
	"fmt"
	"time"

	"github.com/henomis/langfuse-go"
	"github.com/henomis/langfuse-go/model"
	"github.com/openai/openai-go"
)

// langfuseTraceIDKey is a context key for storing the LangFuse Trace ID.
// This should match the key used in ask.go
type langfuseTraceIDKey struct{}

// langfuseGenerationKey is a context key for storing the LangFuse Generation observation.
type langfuseGenerationKey struct{}

// langfusePlugin holds the LangFuse lfClient instance.
type langfusePlugin struct {
	lfClient *langfuse.Langfuse
}

// LangfusePlugin creates a goai-kit plugin that integrates with LangFuse.
// It requires an initialized LangFuse lfClient.
func LangfusePlugin(lfClient *langfuse.Langfuse) Plugin {
	return func() []ClientOption {
		p := &langfusePlugin{lfClient: lfClient}

		return []ClientOption{
			func(config *Config) {
				config.lf = lfClient // Store the LangFuse client in the config
			},

			WithBeforeRequestHook(p.beforeRequestHook),
			WithAfterRequestHook(p.afterRequestHook),
		}
	}
}

// beforeRequestHook creates a LangFuse Trace or Span and a Generation observation.
// It adds the Trace/Span ID and the Generation observation to the context.
func (p *langfusePlugin) beforeRequestHook(
	ctx *Context,
	params openai.ChatCompletionNewParams,
) openai.ChatCompletionNewParams {
	ctx.logger.Debug("LangFuse beforeRequestHook called", "has_client", p.lfClient != nil)
	if p.lfClient == nil {
		return params
	}

	traceID, ok := ctx.Value(langfuseTraceIDKey{}).(string)

	if !ok || traceID == "" {
		trace, traceErr := p.lfClient.Trace(&model.Trace{
			Name:  "goaikit-openai-trace",
			Input: params,
		})

		if traceErr == nil {
			traceID = trace.ID
			ctx.WithValue(langfuseTraceIDKey{}, trace.ID)
		} else {
			ctx.logger.Error("LangFuse Error: Failed to create Trace",
				"error", traceErr,
			)

			return params
		}
	}

	name := "openai-chat-completion"
	if ctx.config.SpanName != "" {
		name = ctx.config.SpanName
	}

	generation, err := p.lfClient.Generation(&model.Generation{
		Name:      name,
		Input:     params,
		Model:     params.Model,
		TraceID:   traceID,
		StartTime: openai.Ptr(time.Now()),
	}, nil)
	if err != nil {
		ctx.logger.Error("LangFuse Error: Failed to create Generation",
			"error", err,
			"trace_id", traceID,
		)

		return params
	}

	ctx.logger.Debug("LangFuse Generation created",
		"generation_id", generation.ID,
		"trace_id", generation.TraceID,
		"start_time", generation.StartTime,
	)

	// Add the generation observation to the context so the after hook can access it
	ctx.WithValue(langfuseGenerationKey{}, generation)

	return params
}

// afterRequestHook updates the LangFuse Generation observation with the response or error.
func (p *langfusePlugin) afterRequestHook(
	ctx *Context,
	response *openai.ChatCompletion,
	err error,
) (*openai.ChatCompletion, error) {
	if p.lfClient == nil {
		return response, err
	}

	gen, ok := ctx.Value(langfuseGenerationKey{}).(*model.Generation)
	if !ok {
		return response, err
	}

	if err != nil {
		gen.Level = model.ObservationLevelError
		gen.StatusMessage = err.Error()
		gen.Output = map[string]any{"error": err.Error()}
	} else if response != nil {
		if len(response.Choices) > 0 {
			gen.Output = response.Choices[0].Message.Content
		}

		gen.Usage = model.Usage{
			PromptTokens:     int(response.Usage.PromptTokens),
			CompletionTokens: int(response.Usage.CompletionTokens),
			TotalTokens:      int(response.Usage.TotalTokens),
		}
	}

	gen.EndTime = openai.Ptr(time.Now())

	_, updateErr := p.lfClient.GenerationEnd(gen)
	if updateErr != nil {
		ctx.logger.Warn("LangFuse Error: Failed to update Generation",
			"error", updateErr,
			"trace_id", gen.TraceID,
			"generation_id", gen.ID,
		)
	}

	return response, err
}

func WithTrace[T any](
	ctx context.Context,
	c *Client,
	trace *model.Trace,
	call func(ctx context.Context) (*T, error),
	modifier ...TraceModifier[T],
) (*T, error) {
	if c.config.lf == nil {
		c.logger.Debug("LangFuse client not configured, skipping trace")

		return call(ctx)
	}

	if trace.ID == "" {
		t, err := c.config.lf.Trace(trace)
		if err != nil {
			return nil, fmt.Errorf("failed to create LangFuse trace: %w", err)
		}

		trace = t
	}

	ctx = context.WithValue(ctx, langfuseTraceIDKey{}, trace.ID)

	c.logger.Debug("Trace created", "trace_id", trace.ID)

	response, err := call(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range modifier {
		t(trace, response)
	}

	if trace.Output == nil {
		trace.Output = response
	}

	if _, err := c.config.lf.Trace(trace); err != nil {
		c.logger.Warn("LangFuse Error: Failed to create Trace",
			"error", err,
			"trace_id", trace.ID,
		)
	}

	return response, nil
}

type TraceModifier[T any] func(t *model.Trace, resp *T)

func WithTraceOutput[T any](f func(t *T) any) TraceModifier[T] {
	return func(t *model.Trace, resp *T) {
		if resp == nil {
			return
		}

		output := f(resp)

		if output != nil {
			t.Output = output
		}
	}
}

func WithSpanName(name string) AskOption {
	return func(config *AskConfig) {
		config.SpanName = name
	}
}
