package kit

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/mhrlife/goai-kit/internal/callback"
	"github.com/mhrlife/goai-kit/internal/schema"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

// Agent represents an AI agent that can execute tasks with tools
type Agent[Output any] struct {
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

// CreateAgent creates a new agent that returns string output
func CreateAgent(client *Client, tools ...ToolExecutor) *Agent[string] {
	return CreateAgentWithOutput[string](client, tools...)
}

// CreateAgentWithOutput creates a new agent with typed output
func CreateAgentWithOutput[Output any](client *Client, tools ...ToolExecutor) *Agent[Output] {
	toolMap := make(map[string]ToolExecutor)
	schemaMap := make(map[string]ToolSchema)

	for _, tool := range tools {
		toolSchema := BuildToolSchema(tool)
		toolMap[toolSchema.ID] = tool
		schemaMap[toolSchema.ID] = toolSchema
	}

	model := "gpt-4o"
	if client.config.DefaultModel != "" {
		model = client.config.DefaultModel
	}

	return &Agent[Output]{
		client:        client,
		tools:         toolMap,
		schemas:       schemaMap,
		model:         model,
		callbacks:     []callback.AgentCallback{},
		maxIterations: 10,
	}
}

// WithModel sets the model for the agent
func (a *Agent[Output]) WithModel(model string) *Agent[Output] {
	a.model = model
	return a
}

// WithCallbacks sets the default callbacks for the agent
func (a *Agent[Output]) WithCallbacks(callbacks ...callback.AgentCallback) *Agent[Output] {
	a.callbacks = callbacks
	return a
}

// WithMaxIterations sets the maximum number of tool calling iterations
func (a *Agent[Output]) WithMaxIterations(max int) *Agent[Output] {
	a.maxIterations = max
	return a
}

// WithTemperature sets the temperature for generation
func (a *Agent[Output]) WithTemperature(temp float64) *Agent[Output] {
	a.temperature = &temp
	return a
}

// Invoke executes the agent with the given configuration
func (a *Agent[Output]) Invoke(ctx context.Context, config InvokeConfig) (Output, error) {
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
	var outputType Output
	hasOutputClass := !isStringType(outputType)

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
	result, iterations, err := a.executeLoop(ctx, messages, cbManager, maxIter)
	if err != nil {
		cbManager.OnError(err, "run")
		return zero, err
	}

	// Trigger OnRunEnd
	cbManager.OnRunEnd(result, iterations)

	return result, nil
}

// mergeCallbacks merges invoke and agent callbacks, prioritizing invoke callbacks
func (a *Agent[Output]) mergeCallbacks(invokeCallbacks []callback.AgentCallback) []callback.AgentCallback {
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
func (a *Agent[Output]) buildMessages(config InvokeConfig) ([]openai.ChatCompletionMessageParamUnion, error) {
	var messages []openai.ChatCompletionMessageParamUnion

	// Add system prompt if provided
	if config.SystemPrompt != "" {
		messages = append(messages, openai.SystemMessage(config.SystemPrompt))
	}

	// Use either Prompt or Messages
	if config.Prompt != "" && len(config.Messages) > 0 {
		return nil, fmt.Errorf("cannot specify both Prompt and Messages")
	}

	if config.Prompt != "" {
		messages = append(messages, openai.UserMessage(config.Prompt))
	} else if len(config.Messages) > 0 {
		messages = append(messages, config.Messages...)
	} else {
		return nil, fmt.Errorf("must specify either Prompt or Messages")
	}

	return messages, nil
}

// executeLoop runs the agent's tool calling loop
func (a *Agent[Output]) executeLoop(
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
		var outputType Output
		if !isStringType(outputType) {
			// Add response format for structured output
			outputSchema := schema.InferJSONSchema(outputType)
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
		finishReason := string(choice.FinishReason)
		content := choice.Message.Content
		toolCalls := choice.Message.ToolCalls

		// Trigger OnGenerationEnd
		cbManager.OnGenerationEnd(finishReason, content, toolCalls, &completion.Usage)

		// Add assistant message to history
		messages = append(messages, choice.Message.ToParam())

		// Check if we're done (no tool calls means we have final response)
		if len(toolCalls) == 0 {
			// Parse output
			if isStringType(outputType) {
				// Return string directly
				return any(content).(Output), iteration, nil
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
		if len(toolCalls) > 0 {
			toolMessages, err := a.executeToolCalls(ctx, toolCalls, cbManager)
			if err != nil {
				cbManager.OnError(err, "tool")
				return zero, iteration, err
			}
			messages = append(messages, toolMessages...)
		}
	}

	err := fmt.Errorf("max iterations (%d) reached without completion", maxIterations)
	cbManager.OnError(err, "run")
	return zero, iteration, err
}

// executeToolCalls executes all tool calls and returns tool messages
func (a *Agent[Output]) executeToolCalls(
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
		if toolValue.Kind() == reflect.Ptr {
			toolValue = toolValue.Elem()
		}

		// Create a new instance of the tool
		toolCopy := reflect.New(toolValue.Type()).Interface().(ToolExecutor)

		// Unmarshal args into the tool copy
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), toolCopy); err != nil {
			cbManager.OnToolCallEnd(toolName, args, nil, toolCallID, err)
			return nil, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
		}

		// Create Context wrapper
		ctxWrapper := &Context{
			Context: ctx,
			logger:  a.client.Logger,
		}

		// Execute tool
		result, err := toolCopy.Execute(ctxWrapper)
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
func isStringType(v interface{}) bool {
	_, ok := any(v).(string)
	return ok
}

// InvokeSimple is a convenience method for simple prompts
func (a *Agent[Output]) InvokeSimple(ctx context.Context, prompt string) (Output, error) {
	return a.Invoke(ctx, InvokeConfig{Prompt: prompt})
}

// InvokeWithMessages is a convenience method for message-based invocation
func (a *Agent[Output]) InvokeWithMessages(
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
) (Output, error) {
	return a.Invoke(ctx, InvokeConfig{Messages: messages})
}

// Client returns the underlying Client
func (a *Agent[Output]) Client() *Client {
	return a.client
}

// Tools returns the agent's tools as a slice
func (a *Agent[Output]) Tools() []ToolExecutor {
	tools := make([]ToolExecutor, 0, len(a.tools))
	for _, tool := range a.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Model returns the agent's model
func (a *Agent[Output]) Model() string {
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
