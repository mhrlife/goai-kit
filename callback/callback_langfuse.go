package callback

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// LangfuseCallback implements AgentCallback using OpenTelemetry for Langfuse tracing
// It properly handles nested observations and trace IDs similar to the PHP implementation
type LangfuseCallback struct {
	BaseCallback

	tracer trace.Tracer

	// Span tracking
	traceSpan             trace.Span
	rootSpan              trace.Span
	currentGenerationSpan trace.Span
	toolSpans             map[string]trace.Span

	// Context management - mimicking Python/PHP's attach/detach pattern
	traceContext    context.Context
	rootSpanContext context.Context

	// Configuration
	serviceName string
	traceID     string
}

// LangfuseCallbackConfig configures the Langfuse callback with OTEL
type LangfuseCallbackConfig struct {
	// Tracer is the OpenTelemetry tracer (required)
	Tracer trace.Tracer

	// ServiceName identifies the service (optional, defaults to "goaikit")
	ServiceName string

	// TraceID allows reusing an existing trace (optional)
	TraceID string

	// ParentContext allows creating child callbacks (optional)
	ParentContext context.Context
}

// NewLangfuseCallback creates a new Langfuse callback handler using OTEL
func NewLangfuseCallback(config LangfuseCallbackConfig) *LangfuseCallback {
	if config.Tracer == nil {
		panic("Tracer is required")
	}

	serviceName := config.ServiceName
	if serviceName == "" {
		serviceName = "goaikit"
	}

	lc := &LangfuseCallback{
		tracer:      config.Tracer,
		serviceName: serviceName,
		traceID:     config.TraceID,
		toolSpans:   make(map[string]trace.Span),
	}

	// Initialize trace span
	lc.initializeTrace(config.TraceID, config.ParentContext)

	return lc
}

// initializeTrace creates the trace span for this execution
// Mimics Python's context attachment pattern
func (lc *LangfuseCallback) initializeTrace(traceID string, parentContext context.Context) {
	ctx := parentContext
	if ctx == nil {
		ctx = context.Background()
	}

	// Start trace span
	lc.traceContext, lc.traceSpan = lc.tracer.Start(
		ctx,
		"trace",
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	// Store trace ID if provided
	if traceID != "" {
		lc.traceSpan.SetAttributes(attribute.String("trace_id", traceID))
		lc.traceID = traceID
	} else {
		lc.traceID = lc.traceSpan.SpanContext().TraceID().String()
	}
}

func (lc *LangfuseCallback) Name() string {
	return "LangfuseCallback"
}

// OnRunStart creates a root span for the agent run
func (lc *LangfuseCallback) OnRunStart(ctx map[string]interface{}) {
	runID := ctx["run_id"].(string)
	parentRunID := lc.getParentRunID(ctx)

	// Only create root span if this is not a nested run
	if parentRunID == "" {
		// Start root span - it will automatically use current context (trace context)
		lc.rootSpanContext, lc.rootSpan = lc.tracer.Start(
			lc.traceContext,
			"agent.run",
			trace.WithSpanKind(trace.SpanKindInternal),
		)

		// Set attributes
		if model, ok := ctx["model"].(string); ok {
			lc.rootSpan.SetAttributes(
				attribute.String("langfuse.observation.model.name", model),
			)
		}

		if input := ctx["input"]; input != nil {
			inputJSON, _ := json.Marshal(input)
			lc.rootSpan.SetAttributes(
				attribute.String("langfuse.observation.input", string(inputJSON)),
			)
		}

		if hasOutputClass, ok := ctx["has_output_class"].(bool); ok && hasOutputClass {
			lc.rootSpan.SetAttributes(
				attribute.Bool("has_structured_output", true),
			)
		}

		lc.rootSpan.SetAttributes(attribute.String("run_id", runID))
	}
}

// OnRunEnd completes the root span with output
func (lc *LangfuseCallback) OnRunEnd(ctx map[string]interface{}) {
	if lc.rootSpan == nil {
		return
	}

	// Set output
	if output := ctx["output"]; output != nil {
		outputJSON, _ := json.Marshal(output)
		lc.rootSpan.SetAttributes(
			attribute.String("langfuse.observation.output", string(outputJSON)),
		)
	}

	// Set total iterations
	if totalIterations, ok := ctx["total_iterations"].(int); ok {
		lc.rootSpan.SetAttributes(
			attribute.Int("total_iterations", totalIterations),
		)
	}

	lc.rootSpan.SetStatus(codes.Ok, "")
	lc.rootSpan.End()

	// End trace span if it exists
	if lc.traceSpan != nil {
		lc.traceSpan.SetStatus(codes.Ok, "")
		lc.traceSpan.End()
	}
}

// OnGenerationStart creates a generation span
func (lc *LangfuseCallback) OnGenerationStart(ctx map[string]interface{}) {
	if lc.rootSpan == nil {
		return
	}

	// Start generation span - will automatically use current context (root span context)
	spanCtx, span := lc.tracer.Start(
		lc.rootSpanContext,
		"llm.generation",
		trace.WithSpanKind(trace.SpanKindClient),
	)

	lc.currentGenerationSpan = span
	_ = spanCtx // We don't need to store this as we're not creating nested children

	// Set attributes
	if model, ok := ctx["model"].(string); ok {
		span.SetAttributes(
			attribute.String("langfuse.observation.model.name", model),
			attribute.String("gen_ai.request.model", model),
		)
	}

	if iteration, ok := ctx["iteration"].(int); ok {
		span.SetAttributes(attribute.Int("iteration", iteration))
	}

	if messages := ctx["messages"]; messages != nil {
		messagesJSON, _ := json.Marshal(messages)
		span.SetAttributes(
			attribute.String("langfuse.observation.input", string(messagesJSON)),
		)
	}
}

// OnGenerationEnd completes the generation span with output and usage
func (lc *LangfuseCallback) OnGenerationEnd(ctx map[string]interface{}) {
	if lc.currentGenerationSpan == nil {
		return
	}

	// Set finish reason
	if finishReason, ok := ctx["finish_reason"].(string); ok {
		lc.currentGenerationSpan.SetAttributes(
			attribute.String("finish_reason", finishReason),
		)
	}

	// Build complete output including tool calls if present
	output := make(map[string]interface{})

	if content, ok := ctx["content"].(string); ok && content != "" {
		output["content"] = content
	}

	// Add tool calls to output if present
	if toolCalls := ctx["tool_calls"]; toolCalls != nil {
		if calls, ok := toolCalls.([]openai.ChatCompletionMessageToolCall); ok && len(calls) > 0 {
			toolCallsData := make([]map[string]interface{}, len(calls))
			for i, call := range calls {
				toolCallsData[i] = map[string]interface{}{
					"id":   call.ID,
					"type": string(call.Type),
					"function": map[string]interface{}{
						"name":      call.Function.Name,
						"arguments": call.Function.Arguments,
					},
				}
			}
			output["tool_calls"] = toolCallsData

			lc.currentGenerationSpan.SetAttributes(
				attribute.Bool("has_tool_calls", true),
				attribute.Int("tool_calls_count", len(calls)),
			)
		}
	}

	// Set output
	outputJSON, _ := json.Marshal(output)
	lc.currentGenerationSpan.SetAttributes(
		attribute.String("langfuse.observation.output", string(outputJSON)),
	)

	// Add usage information if available
	if usage := ctx["usage"]; usage != nil {
		if u, ok := usage.(*openai.CompletionUsage); ok {
			usageDetails := map[string]interface{}{
				"prompt_tokens":     int(u.PromptTokens),
				"completion_tokens": int(u.CompletionTokens),
				"total_tokens":      int(u.TotalTokens),
			}
			usageJSON, _ := json.Marshal(usageDetails)
			lc.currentGenerationSpan.SetAttributes(
				attribute.String("langfuse.observation.usage_details", string(usageJSON)),
			)
		}
	}

	lc.currentGenerationSpan.SetStatus(codes.Ok, "")
	lc.currentGenerationSpan.End()
	lc.currentGenerationSpan = nil
}

// OnToolCallStart creates a span for tool execution
func (lc *LangfuseCallback) OnToolCallStart(ctx map[string]interface{}) {
	if lc.rootSpan == nil {
		return
	}

	toolName, _ := ctx["tool_name"].(string)
	toolCallID, _ := ctx["tool_call_id"].(string)

	// Start tool span - will automatically use current context (root span context)
	_, toolSpan := lc.tracer.Start(
		lc.rootSpanContext,
		fmt.Sprintf("tool.%s", toolName),
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	toolSpan.SetAttributes(
		attribute.String("tool.name", toolName),
		attribute.String("tool_call_id", toolCallID),
	)

	if arguments := ctx["arguments"]; arguments != nil {
		argsJSON, _ := json.Marshal(arguments)
		toolSpan.SetAttributes(
			attribute.String("langfuse.observation.input", string(argsJSON)),
		)
	}

	lc.toolSpans[toolCallID] = toolSpan
}

// OnToolCallEnd completes the tool span with result
func (lc *LangfuseCallback) OnToolCallEnd(ctx map[string]interface{}) {
	toolCallID, ok := ctx["tool_call_id"].(string)
	if !ok {
		return
	}

	toolSpan, exists := lc.toolSpans[toolCallID]
	if !exists {
		return
	}

	// Set output
	if result := ctx["result"]; result != nil {
		resultJSON, _ := json.Marshal(result)
		toolSpan.SetAttributes(
			attribute.String("langfuse.observation.output", string(resultJSON)),
		)
	}

	// Check for error
	if errVal, hasError := ctx["error"]; hasError && errVal != nil {
		errMsg := errVal.(string)
		toolSpan.SetStatus(codes.Error, errMsg)
		toolSpan.RecordError(fmt.Errorf("%s", errMsg))
	} else {
		toolSpan.SetStatus(codes.Ok, "")
	}

	toolSpan.End()
	delete(lc.toolSpans, toolCallID)
}

// OnError handles errors by ending all open spans
func (lc *LangfuseCallback) OnError(ctx map[string]interface{}) {
	errMsg, _ := ctx["error"].(string)
	err := fmt.Errorf("%s", errMsg)

	// End current generation span with error
	if lc.currentGenerationSpan != nil {
		lc.currentGenerationSpan.RecordError(err)
		lc.currentGenerationSpan.SetStatus(codes.Error, errMsg)
		lc.currentGenerationSpan.End()
		lc.currentGenerationSpan = nil
	}

	// End all tool spans with error
	for toolCallID, toolSpan := range lc.toolSpans {
		toolSpan.RecordError(err)
		toolSpan.SetStatus(codes.Error, errMsg)
		toolSpan.End()
		delete(lc.toolSpans, toolCallID)
	}

	// End root span with error
	if lc.rootSpan != nil {
		lc.rootSpan.RecordError(err)
		lc.rootSpan.SetStatus(codes.Error, errMsg)
		lc.rootSpan.End()
		lc.rootSpan = nil
	}

	// End trace span with error
	if lc.traceSpan != nil {
		lc.traceSpan.RecordError(err)
		lc.traceSpan.SetStatus(codes.Error, errMsg)
		lc.traceSpan.End()
		lc.traceSpan = nil
	}
}

// Helper methods

// getParentRunID extracts parent_run_id from context
func (lc *LangfuseCallback) getParentRunID(ctx map[string]interface{}) string {
	if parentID, exists := ctx["parent_run_id"]; exists && parentID != nil {
		return parentID.(string)
	}
	return ""
}

// GetTraceContext returns the current trace context for creating child callbacks
func (lc *LangfuseCallback) GetTraceContext() context.Context {
	return lc.traceContext
}

// GetTraceID returns the current trace ID
func (lc *LangfuseCallback) GetTraceID() string {
	return lc.traceID
}

// GetTraceURL returns the URL to view the trace in Langfuse
func (lc *LangfuseCallback) GetTraceURL(langfuseHost string) string {
	if lc.traceID == "" {
		return ""
	}
	if langfuseHost == "" {
		langfuseHost = "https://cloud.langfuse.com"
	}
	return fmt.Sprintf("%s/trace/%s", langfuseHost, lc.traceID)
}
