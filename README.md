# GoAI Kit

A simple, no-magic Go library for interacting with OpenAI-compatible LLMs. Get structured JSON, plain text, or use tools
with minimal boilerplate.

## Installation

```bash
go get github.com/mhrlife/goai-kit
```

## Features

### 1. Typed JSON Responses

Define a Go struct, and `goai-kit` will handle prompting for JSON and unmarshaling the response. You can use
`jsonschema` struct tags to guide the model's output.

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mhrlife/goai-kit/internal/kit"
)

// Define your desired output structure
type MyOutput struct {
	Message string `json:"message" jsonschema:"description=A greeting message"`
	Value   int    `json:"value" jsonschema:"required"`
}

func main() {
	// Create a client
	client := kit.NewClient(kit.WithDefaultModel("gpt-4o-mini"))

	// Create agent with typed output
	agent := kit.CreateAgentWithOutput[MyOutput](client)

	// Get a structured response
	output, err := agent.Invoke(context.Background(), kit.InvokeConfig{
		Prompt: "Say hello and give me the number 42.",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println(output.Message, output.Value) // "Hello", 42
}
```

### 2. Plain String Responses

For simple text generation, use the default agent which returns a string.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mhrlife/goai-kit/internal/kit"
)

func main() {
	client := kit.NewClient(kit.WithDefaultModel("gpt-4o-mini"))

	// Create a simple agent that returns strings
	agent := kit.CreateAgent(client)

	// Get a plain string response
	joke, err := agent.Invoke(context.Background(), kit.InvokeConfig{
		Prompt: "Tell me a short joke.",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println(joke)
}
```

### 3. Agents with Tools

Create an agent with tools to handle complex, multi-step interactions. Implement the `ToolExecutor` interface for your
tools and let the agent handle tool orchestration.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mhrlife/goai-kit/internal/kit"
)

// 1. Define your tool by implementing ToolExecutor interface
type AverageNumbersTool struct {
	kit.BaseTool
	Numbers []float64 `json:"numbers" jsonschema:"description=List of numbers to calculate average"`
}

// Return tool metadata
func (t *AverageNumbersTool) AgentToolInfo() kit.AgentToolInfo {
	return kit.AgentToolInfo{
		Name:        "average_numbers",
		Description: "Calculate the average of a list of numbers.",
	}
}

// Execute the tool logic
func (t *AverageNumbersTool) Execute(ctx *kit.Context) (any, error) {
	if len(t.Numbers) == 0 {
		return map[string]interface{}{"average": 0.0}, nil
	}

	sum := 0.0
	for _, num := range t.Numbers {
		sum += num
	}
	average := sum / float64(len(t.Numbers))

	return map[string]interface{}{"average": average}, nil
}

func main() {
	// 2. Create client
	client := kit.NewClient(
		kit.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
		kit.WithDefaultModel("gpt-4o-mini"),
	)

	// 3. Create agent with tools
	agent := kit.CreateAgent(client, &AverageNumbersTool{})

	// 4. Invoke agent
	result, err := agent.Invoke(context.Background(), kit.InvokeConfig{
		Prompt: "What is the average of the numbers 10, 20, 30, 40, and 50?",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("Result:", result)
}
```

### 4. File & Image Uploads

Send files (PDFs, images) for multimodal analysis with agents.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mhrlife/goai-kit/internal/kit"
)

type ImageAnalysis struct {
	Description string `json:"description"`
}

func main() {
	// Read image bytes
	imageBytes, err := os.ReadFile("image.png")
	if err != nil {
		log.Fatalf("Error reading image: %v", err)
	}

	client := kit.NewClient(
		kit.WithDefaultModel("gpt-4o-mini"), // or use a model that supports vision
	)

	// Create agent with typed output
	agent := kit.CreateAgentWithOutput[ImageAnalysis](client)

	// Analyze the image
	analysis, err := agent.InvokeWithMessages(context.Background(), []interface{}{
		map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]string{
				"url": "data:image/png;base64," + string(imageBytes),
			},
		},
	}, kit.InvokeConfig{
		Prompt: "What is in this image?",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("Description:", analysis.Description)
}
```

### 5. Dynamic Prompts with Go Templates

`goai-kit` supports Go's built-in `text/template` engine to create dynamic prompts. This allows you to separate your
prompt logic from your application code and build complex prompt structures with conditions and loops.

**1. Create your template file**

Create a file with a `.tpl` extension (e.g., `prompts/hello.tpl`):

```gotemplate
{{if .Context.Ready}}Ready: {{end}}Hello {{ .Data.Name }}
```

The template has access to a `Render` struct containing:

- `.Context`: A custom, typed struct you define for controlling template logic (e.g., flags, user state).
- `.Data`: A `map[string]any` or any other struct for injecting dynamic data into the prompt.

**2. Load and execute the template in your Go code**

Use Go's `embed` package to load your templates and then use the `Template` manager to execute them.

```go
import (
"context"
"embed"
"fmt"
"log"
"github.com/mhrlife/goai-kit"
)

//go:embed prompts/*.tpl
var promptTemplates embed.FS

// Define a context for your templates
type PromptContext struct {
Ready bool
}

func main() {
// 1. Create a new template manager
tpl := prompt.NewTemplate[PromptContext]()

// 2. Load templates from the embedded filesystem.
// This assumes your templates are in a 'prompts' directory.
err := tpl.Load(promptTemplates)
if err != nil {
log.Fatal(err)
}

// 3. Execute the template to generate a prompt
prompt, err := tpl.Execute("hello", prompt.Render[PromptContext]{
Context: PromptContext{Ready: true},
Data:    map[string]any{"Name": "Amir"},
})
if err != nil {
log.Fatal(err)
}

fmt.Println(prompt)
// Output: Ready: Hello Amir
```

### 6. OTEL Langfuse Integration for Agent Tracing

Monitor and debug your agents with OTEL-based tracing using Langfuse. Track agent invocations, tool executions, and
model calls automatically.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mhrlife/goai-kit/internal/callback"
	"github.com/mhrlife/goai-kit/internal/kit"
	"github.com/mhrlife/goai-kit/internal/tracing"
)

// Define your tool (same as before)
type AverageNumbersTool struct {
	kit.BaseTool
	Numbers []float64 `json:"numbers" jsonschema:"description=List of numbers to calculate average"`
}

func (t *AverageNumbersTool) AgentToolInfo() kit.AgentToolInfo {
	return kit.AgentToolInfo{
		Name:        "average_numbers",
		Description: "Calculate the average of a list of numbers.",
	}
}

func (t *AverageNumbersTool) Execute(ctx *kit.Context) (any, error) {
	if len(t.Numbers) == 0 {
		return map[string]interface{}{"average": 0.0}, nil
	}

	sum := 0.0
	for _, num := range t.Numbers {
		sum += num
	}
	average := sum / float64(len(t.Numbers))

	return map[string]interface{}{"average": average}, nil
}

func main() {
	// 1. Initialize OTEL Langfuse tracer
	tracer, err := tracing.NewOTELLangfuseTracer(tracing.LangfuseConfig{
		SecretKey:   os.Getenv("LANGFUSE_SECRET_KEY"),
		PublicKey:   os.Getenv("LANGFUSE_PUBLIC_KEY"),
		Host:        "cloud.langfuse.com",
		URLPath:     "/api/public/otel/v1/traces",
		Environment: "development",
	})
	if err != nil {
		panic(err)
	}
	defer tracer.FlushOrPanic()

	// 2. Create client
	client := kit.NewClient(
		kit.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
		kit.WithDefaultModel("gpt-4o-mini"),
	)

	// 3. Create agent with tools and add Langfuse callback
	agent := kit.CreateAgent(client, &AverageNumbersTool{}).
		WithCallbacks(callback.NewLangfuseCallback(callback.LangfuseCallbackConfig{
			Tracer:      tracer.Tracer(),
			ServiceName: "average-calculator",
		}))

	// 4. Invoke agent - all calls are automatically traced
	result, err := agent.Invoke(context.Background(), kit.InvokeConfig{
		Prompt: "What is the average of the numbers 10, 20, 30, 40, and 50?",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("Result:", result)
	fmt.Println("Trace available in Langfuse dashboard!")
}
```
