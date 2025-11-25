package callback

import (
	"github.com/google/uuid"
	"github.com/openai/openai-go"
)

type Manager struct {
	callbacks     []AgentCallback
	runID         string
	parentRunID   *string
	nestedRunID   map[string]string // tool_call_id -> nested_run_id for nested tool executions
	nestedParents map[string]string // nested_run_id -> parent_run_id
}

// NewManager creates a new callback manager
func NewManager(callbacks []AgentCallback, parentRunID *string) *Manager {
	return &Manager{
		callbacks:     callbacks,
		runID:         uuid.New().String(),
		parentRunID:   parentRunID,
		nestedRunID:   make(map[string]string),
		nestedParents: make(map[string]string),
	}
}

// createNestedRun creates a nested run ID for tool execution
func (cm *Manager) createNestedRun(toolCallID string) string {
	nestedID := uuid.New().String()
	cm.nestedRunID[toolCallID] = nestedID
	cm.nestedParents[nestedID] = cm.runID
	return nestedID
}

// getNestedRunID gets the nested run ID for a tool call
func (cm *Manager) getNestedRunID(toolCallID string) *string {
	if id, ok := cm.nestedRunID[toolCallID]; ok {
		return &id
	}
	return nil
}

// addRunContext adds run_id and parent_run_id to context
func (cm *Manager) addRunContext(ctx map[string]interface{}, nestedRunID *string) map[string]interface{} {
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

// OnRunStart triggers OnRunStart for all callbacks
func (cm *Manager) OnRunStart(model string, input interface{}, hasOutputClass bool) {
	ctx := cm.addRunContext(map[string]interface{}{
		"model":            model,
		"input":            input,
		"has_output_class": hasOutputClass,
	}, nil)

	for _, cb := range cm.callbacks {
		cb.OnRunStart(ctx)
	}
}

// OnRunEnd triggers OnRunEnd for all callbacks
func (cm *Manager) OnRunEnd(output interface{}, totalIterations int) {
	ctx := cm.addRunContext(map[string]interface{}{
		"output":           output,
		"total_iterations": totalIterations,
	}, nil)

	for _, cb := range cm.callbacks {
		cb.OnRunEnd(ctx)
	}
}

// OnGenerationStart triggers OnGenerationStart for all callbacks
func (cm *Manager) OnGenerationStart(
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

// OnGenerationEnd triggers OnGenerationEnd for all callbacks
func (cm *Manager) OnGenerationEnd(
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

// OnToolCallStart triggers OnToolCallStart for all callbacks
func (cm *Manager) OnToolCallStart(toolName string, arguments map[string]interface{}, toolCallID string) {
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

// OnToolCallEnd triggers OnToolCallEnd for all callbacks
func (cm *Manager) OnToolCallEnd(
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

// OnError triggers OnError for all callbacks
func (cm *Manager) OnError(err error, stage string) {
	ctx := cm.addRunContext(map[string]interface{}{
		"error": err.Error(),
		"stage": stage,
	}, nil)

	for _, cb := range cm.callbacks {
		cb.OnError(ctx)
	}
}
