package callback

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
