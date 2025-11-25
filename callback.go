package goaikit

import (
	"github.com/google/uuid"
	"github.com/openai/openai-go"
)

// AgentCallback defines the interface for agent lifecycle callbacks
// Similar to LangChain's callback system for observability and tracing
type AgentCallback interface {
	Name() string
	// OnRunStart is called when the agent starts execution
	// Context contains: model, input, has_output_class, run_id, parent_run_id
	OnRunStart(ctx map[string]interface{})

	// OnRunEnd is called when the agent completes execution
	// Context contains: output, total_iterations, run_id, parent_run_id
	OnRunEnd(ctx map[string]interface{})

	// OnGenerationStart is called before each LLM API call
	// Context contains: iteration, messages, model, run_id, parent_run_id
	OnGenerationStart(ctx map[string]interface{})

	// OnGenerationEnd is called after each LLM API call
	// Context contains: finish_reason, content, tool_calls, usage, run_id, parent_run_id
	OnGenerationEnd(ctx map[string]interface{})

	// OnToolCallStart is called before tool execution
	// Context contains: tool_name, arguments, tool_call_id, run_id, parent_run_id
	OnToolCallStart(ctx map[string]interface{})

	// OnToolCallEnd is called after tool execution
	// Context contains: tool_name, arguments, result, tool_call_id, run_id, parent_run_id, error (if any)
	OnToolCallEnd(ctx map[string]interface{})

	// OnError is called when an error occurs
	// Context contains: error, stage (run/generation/tool), run_id, parent_run_id
	OnError(ctx map[string]interface{})
}

// BaseCallback provides empty implementations for all callback methods
// Embed this in your callback to only override methods you need
type BaseCallback struct{}

func (b *BaseCallback) OnRunStart(ctx map[string]interface{})        {}
func (b *BaseCallback) OnRunEnd(ctx map[string]interface{})          {}
func (b *BaseCallback) OnGenerationStart(ctx map[string]interface{}) {}
func (b *BaseCallback) OnGenerationEnd(ctx map[string]interface{})   {}
func (b *BaseCallback) OnToolCallStart(ctx map[string]interface{})   {}
func (b *BaseCallback) OnToolCallEnd(ctx map[string]interface{})     {}
func (b *BaseCallback) OnError(ctx map[string]interface{})           {}

// callbackManager manages multiple callbacks and provides helper methods
type callbackManager struct {
	callbacks     []AgentCallback
	runID         string
	parentRunID   *string
	nestedRunID   map[string]string // tool_call_id -> nested_run_id for nested tool executions
	nestedParents map[string]string // nested_run_id -> parent_run_id
}

// newCallbackManager creates a new callback manager
func newCallbackManager(callbacks []AgentCallback, parentRunID *string) *callbackManager {
	return &callbackManager{
		callbacks:     callbacks,
		runID:         uuid.New().String(),
		parentRunID:   parentRunID,
		nestedRunID:   make(map[string]string),
		nestedParents: make(map[string]string),
	}
}

// createNestedRun creates a nested run ID for tool execution
func (cm *callbackManager) createNestedRun(toolCallID string) string {
	nestedID := uuid.New().String()
	cm.nestedRunID[toolCallID] = nestedID
	cm.nestedParents[nestedID] = cm.runID
	return nestedID
}

// getNestedRunID gets the nested run ID for a tool call
func (cm *callbackManager) getNestedRunID(toolCallID string) *string {
	if id, ok := cm.nestedRunID[toolCallID]; ok {
		return &id
	}
	return nil
}

// addRunContext adds run_id and parent_run_id to context
func (cm *callbackManager) addRunContext(ctx map[string]interface{}, nestedRunID *string) map[string]interface{} {
	if ctx == nil {
		ctx = make(map[string]interface{})
	}

	if nestedRunID != nil {
		ctx["run_id"] = *nestedRunID
		ctx["parent_run_id"] = cm.runID
	} else {
		ctx["run_id"] = cm.runID
		if cm.parentRunID != nil {
			ctx["parent_run_id"] = *cm.parentRunID
		}
	}

	return ctx
}

// onRunStart triggers OnRunStart for all callbacks
func (cm *callbackManager) onRunStart(model string, input interface{}, hasOutputClass bool) {
	ctx := cm.addRunContext(map[string]interface{}{
		"model":            model,
		"input":            input,
		"has_output_class": hasOutputClass,
	}, nil)

	for _, cb := range cm.callbacks {
		cb.OnRunStart(ctx)
	}
}

// onRunEnd triggers OnRunEnd for all callbacks
func (cm *callbackManager) onRunEnd(output interface{}, totalIterations int) {
	ctx := cm.addRunContext(map[string]interface{}{
		"output":           output,
		"total_iterations": totalIterations,
	}, nil)

	for _, cb := range cm.callbacks {
		cb.OnRunEnd(ctx)
	}
}

// onGenerationStart triggers OnGenerationStart for all callbacks
func (cm *callbackManager) onGenerationStart(
	iteration int,
	messages []openai.ChatCompletionMessageParamUnion,
	model string,
) {
	ctx := cm.addRunContext(map[string]interface{}{
		"iteration": iteration,
		"messages":  messages,
		"model":     model,
	}, nil)

	for _, cb := range cm.callbacks {
		cb.OnGenerationStart(ctx)
	}
}

// onGenerationEnd triggers OnGenerationEnd for all callbacks
func (cm *callbackManager) onGenerationEnd(
	finishReason string,
	content string,
	toolCalls []openai.ChatCompletionMessageToolCall,
	usage *openai.CompletionUsage,
) {
	ctx := cm.addRunContext(map[string]interface{}{
		"finish_reason": finishReason,
		"content":       content,
		"tool_calls":    toolCalls,
		"usage":         usage,
	}, nil)

	for _, cb := range cm.callbacks {
		cb.OnGenerationEnd(ctx)
	}
}

// onToolCallStart triggers OnToolCallStart for all callbacks
func (cm *callbackManager) onToolCallStart(toolName string, arguments map[string]interface{}, toolCallID string) {
	nestedRunID := cm.createNestedRun(toolCallID)
	ctx := cm.addRunContext(map[string]interface{}{
		"tool_name":    toolName,
		"arguments":    arguments,
		"tool_call_id": toolCallID,
	}, &nestedRunID)

	for _, cb := range cm.callbacks {
		cb.OnToolCallStart(ctx)
	}
}

// onToolCallEnd triggers OnToolCallEnd for all callbacks
func (cm *callbackManager) onToolCallEnd(
	toolName string,
	arguments map[string]interface{},
	result interface{},
	toolCallID string,
	err error,
) {
	nestedRunID := cm.getNestedRunID(toolCallID)
	ctx := cm.addRunContext(map[string]interface{}{
		"tool_name":    toolName,
		"arguments":    arguments,
		"result":       result,
		"tool_call_id": toolCallID,
	}, nestedRunID)

	if err != nil {
		ctx["error"] = err.Error()
	}

	for _, cb := range cm.callbacks {
		cb.OnToolCallEnd(ctx)
	}
}

// onError triggers OnError for all callbacks
func (cm *callbackManager) onError(err error, stage string) {
	ctx := cm.addRunContext(map[string]interface{}{
		"error": err.Error(),
		"stage": stage,
	}, nil)

	for _, cb := range cm.callbacks {
		cb.OnError(ctx)
	}
}
