package kit

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"

	"github.com/mhrlife/goai-kit/callback"
	"github.com/mhrlife/goai-kit/schema"
)

// Agent is a configured way of calling a model: a set of tools, a model name, the
// callbacks to notify and the limits of the tool-calling loop. The output type is
// not part of it. It belongs to the call, so one agent can answer as a string
// once and as a struct the next time without being rebuilt:
//
//	agent := client.Agent(&SearchTool{}).WithModel("gpt-4o")
//	text, err := agent.Ask(ctx, "Summarize the release notes")
//	review, err := agent.InvokeSimple[Review](ctx, "Review this PR")
type Agent struct {
	client        *Client
	tools         map[string]ToolExecutor // toolID -> ToolExecutor
	schemas       map[string]ToolSchema   // toolID -> ToolSchema
	model         string
	callbacks     []callback.AgentCallback
	maxIterations int
	temperature   *float64
}

// InvokeConfig contains configuration for agent invocation
type InvokeConfig struct {
	// Prompt is a simple string prompt (mutually exclusive with Messages)
	Prompt string

	// Messages is a list of OpenAI chat completion messages (mutually exclusive with Prompt)
	Messages []openai.ChatCompletionMessageParamUnion

	// Callbacks to be notified of agent lifecycle events
	Callbacks []callback.AgentCallback

	// ParentRunID for nested agent calls (optional)
	ParentRunID *string

	// SystemPrompt to prepend to messages (optional)
	SystemPrompt string

	// MaxIterations for tool calling loop (optional, defaults to agent's maxIterations)
	MaxIterations *int
}

// Agent returns an agent that calls through this client with the given tools.
// This is the usual entry point; NewAgent does the same thing as a function.
func (c *Client) Agent(tools ...ToolExecutor) *Agent {
	toolMap := make(map[string]ToolExecutor)
	schemaMap := make(map[string]ToolSchema)

	for _, tool := range tools {
		toolSchema := BuildToolSchema(tool)
		toolMap[toolSchema.ID] = tool
		schemaMap[toolSchema.ID] = toolSchema
	}

	model := "gpt-4o"
	if c.config.DefaultModel != "" {
		model = c.config.DefaultModel
	}

	return &Agent{
		client:        c,
		tools:         toolMap,
		schemas:       schemaMap,
		model:         model,
		callbacks:     []callback.AgentCallback{},
		maxIterations: 10,
	}
}

// NewAgent is client.Agent as a function, for call sites that already hold the
// client in a variable of their own.
func NewAgent(client *Client, tools ...ToolExecutor) *Agent {
	return client.Agent(tools...)
}

// WithModel sets the model for the agent
func (a *Agent) WithModel(model string) *Agent {
	a.model = model
	return a
}

// WithCallbacks sets the default callbacks for the agent
func (a *Agent) WithCallbacks(callbacks ...callback.AgentCallback) *Agent {
	a.callbacks = callbacks
	return a
}

// WithMaxIterations sets the maximum number of tool calling iterations
func (a *Agent) WithMaxIterations(n int) *Agent {
	a.maxIterations = n
	return a
}

// WithTemperature sets the temperature for generation
func (a *Agent) WithTemperature(temp float64) *Agent {
	a.temperature = &temp
	return a
}

// Invoke executes the agent and decodes the final message into Output. Name the
// type at the call site:
//
//	review, err := agent.Invoke[Review](ctx, kit.InvokeConfig{Prompt: p})
//
// An Output of string returns the message as it came back. Any other type is
// sent to the model as a JSON schema in response_format and unmarshaled from
// the reply, so it must be a shape encoding/json can decode into.
func (a *Agent) Invoke[Output any](ctx context.Context, config InvokeConfig) (Output, error) {
	var zero Output

	// merge all callbacks but when there are two callbacks with the same name, only keep
	// the invoke callback
	allCallbacks := a.mergeCallbacks(config.Callbacks)

	// Create callback manager
	cbManager := callback.NewManager(allCallbacks, config.ParentRunID)

	// Build messages
	messages, err := a.buildMessages(config)
	if err != nil {
		cbManager.OnError(err, "run")
		return zero, err
	}

	// Determine if we have a typed output
	hasOutputClass := !isStringType(zero)

	// Trigger OnRunStart
	input := config.Prompt
	if config.Prompt == "" {
		input = "messages"
	}
	cbManager.OnRunStart(a.model, input, hasOutputClass)

	// Determine max iterations
	maxIter := a.maxIterations
	if config.MaxIterations != nil {
		maxIter = *config.MaxIterations
	}

	// Execute the agent loop
	result, iterations, err := a.executeLoop[Output](ctx, messages, cbManager, maxIter)
	if err != nil {
		cbManager.OnError(err, "run")
		return zero, err
	}

	// Trigger OnRunEnd
	cbManager.OnRunEnd(result, iterations)

	return result, nil
}

// InvokeSimple is a convenience method for simple prompts
func (a *Agent) InvokeSimple[Output any](ctx context.Context, prompt string) (Output, error) {
	return a.Invoke[Output](ctx, InvokeConfig{Prompt: prompt})
}

// InvokeWithMessages is a convenience method for message-based invocation
func (a *Agent) InvokeWithMessages[Output any](
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
) (Output, error) {
	return a.Invoke[Output](ctx, InvokeConfig{Messages: messages})
}

// Ask runs a prompt and returns the reply as text. It is Invoke[string] under a
// shorter name, for the common case where there is no schema to fill.
func (a *Agent) Ask(ctx context.Context, prompt string) (string, error) {
	return a.Invoke[string](ctx, InvokeConfig{Prompt: prompt})
}

// mergeCallbacks merges invoke and agent callbacks, prioritizing invoke callbacks
func (a *Agent) mergeCallbacks(invokeCallbacks []callback.AgentCallback) []callback.AgentCallback {
	allCallbacks := make([]callback.AgentCallback, 0)
	allCallbacks = append(allCallbacks, invokeCallbacks...)
	seenCallbackNames := map[string]struct{}{}
	for _, cb := range invokeCallbacks {
		seenCallbackNames[cb.Name()] = struct{}{}
	}
	for _, cb := range a.callbacks {
		if _, seen := seenCallbackNames[cb.Name()]; !seen {
			allCallbacks = append(allCallbacks, cb)
		}
	}
	return allCallbacks
}

// buildMessages constructs the message list from InvokeConfig
func (a *Agent) buildMessages(config InvokeConfig) ([]openai.ChatCompletionMessageParamUnion, error) {
	var messages []openai.ChatCompletionMessageParamUnion

	// Add system prompt if provided
	if config.SystemPrompt != "" {
		messages = append(messages, openai.SystemMessage(config.SystemPrompt))
	}

	// Use either Prompt or Messages
	if config.Prompt != "" && len(config.Messages) > 0 {
		return nil, fmt.Errorf("cannot specify both Prompt and Messages")
	}

	switch {
	case config.Prompt != "":
		messages = append(messages, openai.UserMessage(config.Prompt))
	case len(config.Messages) > 0:
		messages = append(messages, config.Messages...)
	default:
		return nil, fmt.Errorf("must specify either Prompt or Messages")
	}

	return messages, nil
}

// executeLoop runs the agent's tool calling loop, decoding the final message into
// Output.
func (a *Agent) executeLoop[Output any](
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
	cbManager *callback.Manager,
	maxIterations int,
) (Output, int, error) {
	var zero Output
	iteration := 0

	// Convert tool schemas to OpenAI tool definitions
	tools := make([]openai.ChatCompletionToolParam, 0, len(a.schemas))
	for _, toolSchema := range a.schemas {
		tools = append(tools, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        toolSchema.Name,
				Description: param.NewOpt(toolSchema.Description),
				Parameters:  toolSchema.JSONSchema,
				Strict:      param.NewOpt(true),
			},
		})
	}

	for iteration < maxIterations {
		iteration++

		// Trigger OnGenerationStart
		cbManager.OnGenerationStart(iteration, messages, a.model)

		// Build request params
		params := openai.ChatCompletionNewParams{
			Model:    a.model,
			Messages: messages,
		}

		if a.temperature != nil {
			params.Temperature = param.NewOpt(*a.temperature)
		}

		// Add tools if available
		if len(tools) > 0 {
			params.Tools = tools
		}

		// Check if Output is a struct type for response_format
		if !isStringType(zero) {
			// Add response format for structured output
			outputSchema := schema.InferJSONSchema(zero)
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
					JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
						Strict: param.NewOpt(true),
						Name:   "response",
						Schema: outputSchema,
					},
				},
			}
		}

		// Call OpenAI API
		completion, err := a.client.client.Chat.Completions.New(ctx, params)
		if err != nil {
			cbManager.OnError(err, "generation")
			return zero, iteration, fmt.Errorf("OpenAI API error: %w", err)
		}

		if len(completion.Choices) == 0 {
			err := fmt.Errorf("no choices in response")
			cbManager.OnError(err, "generation")
			return zero, iteration, err
		}

		choice := completion.Choices[0]
		finishReason := choice.FinishReason
		content := choice.Message.Content
		toolCalls := choice.Message.ToolCalls

		// Trigger OnGenerationEnd
		cbManager.OnGenerationEnd(finishReason, content, toolCalls, &completion.Usage)

		// Add assistant message to history
		messages = append(messages, choice.Message.ToParam())

		// Check if we're done (no tool calls means we have final response)
		if len(toolCalls) == 0 {
			// Parse output
			if out, ok := any(content).(Output); ok {
				// Output is string: hand back the message as it came.
				return out, iteration, nil
			}

			// Parse JSON for structured output
			var result Output
			if err := json.Unmarshal([]byte(content), &result); err != nil {
				cbManager.OnError(err, "generation")
				return zero, iteration, fmt.Errorf("failed to parse output JSON: %w", err)
			}
			return result, iteration, nil
		}

		// Execute tool calls
		toolMessages, err := a.executeToolCalls(ctx, toolCalls, cbManager)
		if err != nil {
			cbManager.OnError(err, "tool")
			return zero, iteration, err
		}
		messages = append(messages, toolMessages...)
	}

	err := fmt.Errorf("max iterations (%d) reached without completion", maxIterations)
	cbManager.OnError(err, "run")
	return zero, iteration, err
}

// executeToolCalls executes all tool calls and returns tool messages
func (a *Agent) executeToolCalls(
	ctx context.Context,
	toolCalls []openai.ChatCompletionMessageToolCall,
	cbManager *callback.Manager,
) ([]openai.ChatCompletionMessageParamUnion, error) {
	var toolMessages []openai.ChatCompletionMessageParamUnion

	// Execute each tool call
	for _, toolCall := range toolCalls {
		toolName := toolCall.Function.Name
		toolCallID := toolCall.ID

		// Parse arguments
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			cbManager.OnToolCallEnd(toolName, args, nil, toolCallID, err)
			return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
		}

		// Trigger OnToolCallStart
		cbManager.OnToolCallStart(toolName, args, toolCallID)

		// Find tool by name in schemas and tools maps
		var foundToolID string
		for id, toolSchema := range a.schemas {
			if toolSchema.Name == toolName {
				foundToolID = id
				break
			}
		}

		if foundToolID == "" {
			err := fmt.Errorf("tool not found: %s", toolName)
			cbManager.OnToolCallEnd(toolName, args, nil, toolCallID, err)
			return nil, err
		}

		executor := a.tools[foundToolID]

		// Create a copy of the tool struct to unmarshal args into
		toolValue := reflect.ValueOf(executor)
		if toolValue.Kind() == reflect.Pointer {
			toolValue = toolValue.Elem()
		}

		// Create a new instance of the tool
		toolCopy, ok := reflect.New(toolValue.Type()).Interface().(ToolExecutor)
		if !ok {
			err := fmt.Errorf("tool %s does not implement ToolExecutor on its pointer type", toolName)
			cbManager.OnToolCallEnd(toolName, args, nil, toolCallID, err)
			return nil, err
		}

		// Unmarshal args into the tool copy
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), toolCopy); err != nil {
			cbManager.OnToolCallEnd(toolName, args, nil, toolCallID, err)
			return nil, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
		}

		// Execute tool
		result, err := toolCopy.Execute(NewContext(ctx, a.client.Logger))
		cbManager.OnToolCallEnd(toolName, args, result, toolCallID, err)

		if err != nil {
			return nil, fmt.Errorf("tool %s failed: %w", toolName, err)
		}

		// Convert result to string
		resultStr, err := resultToString(result)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool result to string: %w", err)
		}

		// Add tool message
		toolMessages = append(toolMessages, openai.ToolMessage(resultStr, toolCallID))
	}

	return toolMessages, nil
}

// resultToString converts tool result to string representation
func resultToString(result interface{}) (string, error) {
	if result == nil {
		return "", nil
	}

	switch v := result.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		// Convert to JSON
		data, err := json.Marshal(result)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

// isStringType checks if a type is string
func isStringType(v any) bool {
	_, ok := v.(string)
	return ok
}

// Client returns the underlying Client
func (a *Agent) Client() *Client {
	return a.client
}

// Tools returns the agent's tools as a slice
func (a *Agent) Tools() []ToolExecutor {
	tools := make([]ToolExecutor, 0, len(a.tools))
	for _, tool := range a.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Model returns the agent's model
func (a *Agent) Model() string {
	return a.model
}

// NewOpenAIClientFromKey creates a new goaikit Client from an API key
// This is a convenience function for users
func NewOpenAIClientFromKey(apiKey string, opts ...option.RequestOption) *Client {
	clientOpts := []ClientOption{
		WithAPIKey(apiKey),
	}

	if len(opts) > 0 {
		clientOpts = append(clientOpts, WithRequestOptions(opts...))
	}

	return NewClient(clientOpts...)
}
