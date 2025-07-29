# GoAI Kit

A simple, no-magic Go library for interacting with OpenAI-compatible LLMs. Get structured JSON, plain text, or use tools with minimal boilerplate.

## Installation

```bash
go get github.com/mhrlife/goai-kit
```

## Features

### 1. Typed JSON Responses

Define a Go struct, and `goai-kit` will handle prompting for JSON and unmarshaling the response. You can use `jsonschema` struct tags to guide the model's output.

```go
// Define your desired output structure
type MyOutput struct {
	Message string `json:"message" jsonschema:"description=A greeting message"`
	Value   int    `json:"value" jsonschema:"required"`
}

// Create a client
client := goaikit.NewClient(goaikit.WithDefaultModel("gpt-4o-mini"))

// Get a structured response
output, err := goaikit.Ask[MyOutput](context.Background(), client,
    goaikit.WithPrompt("Say hello and give me the number 42."),
)

fmt.Println(output.Message, output.Value) // "Hello", 42
```

### 2. Plain String Responses

For simple text generation, just ask for a `string`.

```go
// Get a plain string response
text, err := goaikit.Ask[string](context.Background(), client,
    goaikit.WithPrompt("Tell me a short joke."),
)

fmt.Println(*text)
```

### 3. Tool Calling (Function Calling)

Define tools that the LLM can use to access external information or perform actions. `goai-kit` handles the multi-turn conversation automatically.

```go
// 1. Define tool arguments
type CitySearchArgs struct {
	Query string `json:"query" jsonschema_description:"The name of the city to search for."`
}

// 2. Define the tool
getCityIDTool := &goaikit.Tool[CitySearchArgs]{
    Name:        "Get City ID",
    Description: "Get the ID of a city based on its name.",
    Runner: func(ctx *goaikit.ToolContext, args CitySearchArgs) (any, error) {
        // Your logic here... e.g., database lookup
        if strings.ToLower(args.Query) == "jamshideh" {
            return "J-17", nil
        }
        return nil, fmt.Errorf("city not found")
    },
}

// 3. Define the final output structure
type CityInfo struct {
    CityName string `json:"city_name"`
    CityID   string `json:"city_id"`
}

// 4. Call Ask with the tool
cityInfo, err := goaikit.Ask[CityInfo](context.Background(), client,
    goaikit.WithPrompt("What is the ID for the city of Jamshideh?"),
    goaikit.WithTool(getCityIDTool),
)

fmt.Println(cityInfo.CityID) // "J-17"
```

### 4. File & Image Uploads

Send files (PDFs, images) for multimodal analysis. You can send PDF files using `goaikit.FilePDF` and images using `goaikit.FileImage`.

```go
// Read image bytes
imageBytes, _ := os.ReadFile("image.png")

type ImageAnalysis struct {
    Description string `json:"description"`
}

// Ask a question about the image
analysis, err := goaikit.Ask[ImageAnalysis](context.Background(), client,
    goaikit.WithPrompt("What is in this image?"),
    goaikit.WithFile(goaikit.FileImage("image/png", imageBytes)), // Pass the MIME type
    goaikit.WithDefaultModel("google/gemini-pro-vision"), // Use a model that supports vision
)
```

### 5. Dynamic Prompts with Go Templates

`goai-kit` supports Go's built-in `text/template` engine to create dynamic prompts. This allows you to separate your prompt logic from your application code and build complex prompt structures with conditions and loops.

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
	tpl := goaikit.NewTemplate[PromptContext]()

	// 2. Load templates from the embedded filesystem.
	// This assumes your templates are in a 'prompts' directory.
	err := tpl.Load(promptTemplates)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Execute the template to generate a prompt
	prompt, err := tpl.Execute("hello", goaikit.Render[PromptContext]{
		Context: PromptContext{Ready: true},
		Data:    map[string]any{"Name": "Amir"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(prompt)
	// Output: Ready: Hello Amir

	// You can then use this dynamic prompt with goaikit.Ask
	// client := goaikit.NewClient()
	// response, err := goaikit.Ask[string](context.Background(), client,
	//     goaikit.WithPrompt(prompt),
	// )
	// ...
}
```

### 6. Graphs for Multi-Step Workflows

`goai-kit` provides a simple Graph feature to orchestrate complex, multi-step workflows that can include loops and conditional logic. Each step in the graph is a `Node` that can modify a shared `Context` and decide which node to execute next. The `Runner` function of a `Node` returns the name of the next node to execute.

You can control the flow using special string constants:
- `goaikit.GraphExit`: Signals the graph to stop execution successfully. It's an alias for an empty string (`""`).
- `goaikit.GraphRetry`: A special value that tells the graph to re-execute the current node immediately. This is useful for polling or retrying an action until a certain condition is met.

Here's an example of a graph that generates random numbers until it finds an even one:

```go
type NumberGraphContext struct {
	CurrentNumber int
	TextualNumber string
}

const (
	StageSuggestNumber = "suggest_number"
	StageCheckOddEven  = "check_odd_even"
	StageConvertToText = "convert_to_text"
)

client := goaikit.NewClient()

suggestNumberNode := goaikit.Node[NumberGraphContext]{
	Name: StageSuggestNumber,
	Runner: func(ctx context.Context, arg goaikit.NodeArg[NumberGraphContext]) (NumberGraphContext, string, error) {
		arg.Context.CurrentNumber = rand.Intn(10) + 1
		fmt.Printf("Suggested number: %d\n", arg.Context.CurrentNumber)
		return arg.Context, StageCheckOddEven, nil
	},
}

checkOddEvenNode := goaikit.Node[NumberGraphContext]{
	Name: StageCheckOddEven,
	Runner: func(ctx context.Context, arg goaikit.NodeArg[NumberGraphContext]) (NumberGraphContext, string, error) {
		if arg.Context.CurrentNumber%2 != 0 {
			fmt.Println("Number is odd, trying again...")
			return arg.Context, StageSuggestNumber, nil
		}
		fmt.Println("Number is even, continuing...")
		return arg.Context, StageConvertToText, nil
	},
}

convertToTextNode := goaikit.Node[NumberGraphContext]{
	Name: StageConvertToText,
	Runner: func(ctx context.Context, arg goaikit.NodeArg[NumberGraphContext]) (NumberGraphContext, string, error) {
		numberMap := map[int]string{2: "two", 4: "four", 6: "six", 8: "eight", 10: "ten"}
		arg.Context.TextualNumber = numberMap[arg.Context.CurrentNumber]
		fmt.Printf("Converted to text: %s\n", arg.Context.TextualNumber)
		return arg.Context, goaikit.GraphExit, nil // Stop the graph
	},
}

graph, err := goaikit.NewGraph("number_game",
	suggestNumberNode,
	checkOddEvenNode,
	convertToTextNode,
)
if err != nil {
	log.Fatal(err)
}

finalContext, err := graph.Run(context.Background(), client, NumberGraphContext{})
if err != nil {
	log.Fatal(err)
}

fmt.Printf("The final number is: %s\n", finalContext.TextualNumber)
```

#### Using `AICallNode` for AI-Powered Steps

For nodes that need to call an LLM, `goai-kit` provides a convenient `AICallNode` wrapper. It simplifies the process by handling the AI call, structured output parsing, and error handling, letting you focus on the core logic.

You only need to provide:
- A `PromptGenerator` function that creates the prompt based on the current graph context. It can return an error, which will halt the graph's execution.
- A `Callback` function that processes the AI's structured output and updates the context.

Here's an example of a two-stage AI graph that first generates a business idea and then refines it:

```go
import (
	"context"
	"fmt"
	"log"
	"github.com/mhrlife/goai-kit"
)

func main() {
	// 1. Define the graph's context
	type AIGraphContext struct {
		InitialIdea string
		RefinedIdea string
	}

	// 2. Setup the client
	client := goaikit.NewClient(goaikit.WithDefaultModel("gpt-4o-mini"))

	// 3. Define the AI nodes

	// Stage 1: Generate a business idea
	type IdeaOutput struct {
		Idea string `json:"idea" jsonschema:"description=A short, innovative business idea."`
	}

	generateIdeaNode := goaikit.NewAICallNode(goaikit.AICallNode[AIGraphContext, IdeaOutput]{
		Name: "generate_idea",
		PromptGenerator: func(graphContext AIGraphContext) (string, error) {
			return "Suggest a new business idea for a tech startup.", nil
		},
		Callback: func(ctx context.Context, arg goaikit.NodeArg[AIGraphContext], aiOutput *IdeaOutput) (AIGraphContext, string, error) {
			arg.Context.InitialIdea = aiOutput.Idea
			fmt.Printf("Generated Idea: %s\n", arg.Context.InitialIdea)
			return arg.Context, "refine_idea", nil // Go to the next node
		},
	})

	// Stage 2: Refine the business idea
	type RefinedIdeaOutput struct {
		RefinedIdea string `json:"refined_idea" jsonschema:"description=A refined version of the business idea, making it more specific."`
	}

	refineIdeaNode := goaikit.NewAICallNode(goaikit.AICallNode[AIGraphContext, RefinedIdeaOutput]{
		Name: "refine_idea",
		PromptGenerator: func(graphContext AIGraphContext) (string, error) {
			// Use the output from the previous stage
			return fmt.Sprintf("Take this business idea and make it more specific and actionable: '%s'", graphContext.InitialIdea), nil
		},
		Callback: func(ctx context.Context, arg goaikit.NodeArg[AIGraphContext], aiOutput *RefinedIdeaOutput) (AIGraphContext, string, error) {
			arg.Context.RefinedIdea = aiOutput.RefinedIdea
			fmt.Printf("Refined Idea: %s\n", arg.Context.RefinedIdea)
			return arg.Context, goaikit.GraphExit, nil // Stop the graph
		},
	})

	// 4. Create and run the graph
	graph, err := goaikit.NewGraph("ai_idea_generator",
		generateIdeaNode,
		refineIdeaNode,
	)
	if err != nil {
		log.Fatal(err)
	}

	finalContext, err := graph.Run(context.Background(), client, AIGraphContext{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Final refined idea: %s\n", finalContext.RefinedIdea)
}
```

## Deep Research Support (Experimental)

Under development. Currently supports OpenAI o3 and o4-mini deep research tasks with structured output (using some tricks), and includes an OpenAI Deep Research compatible MCP server that has been tested.

### MCP Server Example

Use the following example (`examples/mcp/main.go`) to start an MCP server locally. You can expose it via ngrok:

```go
package main

import (
    goaikit "github.com/mhrlife/goai-kit"
    "log/slog"
)

func main() {
    s1, err := goaikit.NewOpenAIDeepResearchMCPServer(
        "capitals",
        "v0.0.1",
        goaikit.OpenAISearch{
            Description: "search for countries",
            Exec: func(query string) ([]goaikit.OpenAISearchResult, error) {
                slog.Info("searching for countries", "query", query)
                return []goaikit.OpenAISearchResult{
                    {ID: "iran", Title: "Iran"},
                    {ID: "france", Title: "France"},
                }, nil
            },
        },
        goaikit.OpenAIFetch{
            Description: "get the country info",
            Exec: func(id string) (*goaikit.OpenAISearchResult, error) {
                if id == "iran" {
                    return &goaikit.OpenAISearchResult{ID: "iran", Title: "Iran", Text: "Iran is a country in Western Asia.", URL: "https://en.wikipedia.org/wiki/Iran"}, nil
                } else if id == "france" {
                    return &goaikit.OpenAISearchResult{ID: "france", Title: "France", Text: "Capital of france is Tehran in this world", URL: "https://en.wikipedia.org/wiki/France"}, nil
                }
                return nil, nil
            },
        },
    )
    if err != nil {
        slog.Error("failed to create MCP server", "error", err)
        return
    }
    if err := goaikit.StartSSEServer(s1, ":8082"); err != nil {
        slog.Error("failed to start SSE server", "error", err)
    }
}
```

Run with ngrok:

```bash
ngrok http 8082
```

### Deep Research Usage Example

call `DeepResearch` with the MCP server URL:

```go
ctx := context.Background()
client := goaikit.NewClient(
    goaikit.WithBaseURL(os.Getenv("OPENAI_API_BASE")),
    goaikit.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
    goaikit.WithDefaultModel("o4-mini-deep-research"),
    goaikit.WithPlugin(goaikit.LangfusePlugin(langfuse.New(ctx))),
)
type Output struct {
    Found  bool
    Result string
}
out, _, err := goaikit.DeepResearch[Output](
    ctx,
    client,
    goaikit.TaskConfig{
        Instructions: "you *MUST* only use the mcp to answer the question.",
        Prompt:       "What is the capital of France in your random world?",
        MCPServers: []responses.ToolMcpParam{
            goaikit.NewApprovedMCPServer(
                "capitals",
                "https://<your-ngrok-url>/default/sse",
            ),
        },
    },
)
```

## Advanced Usage

### Using Different Providers

`goai-kit` works with any OpenAI-compatible API. Configure the client with a base URL and API key.

**Example: Google Gemini**
```go
client := goaikit.NewClient(
    goaikit.WithAPIKey(os.Getenv("GEMINI_API_KEY")),
    goaikit.WithBaseURL("https://generativelanguage.googleapis.com/v1beta/openai/"),
    goaikit.WithDefaultModel("gemini-1.5-flash"),
)
```

**Example: OpenRouter**
```go
client := goaikit.NewClient(
    goaikit.WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
    goaikit.WithBaseURL(os.Getenv("OPENROUTER_API_BASE")),
    goaikit.WithDefaultModel("openai/gpt-4o-mini"),
)
```

### OpenRouter Specific Features

When using [OpenRouter](https://openrouter.ai/), you can use special options to control model routing and file parsing.

**Model Routing**

Force OpenRouter to select from a specific list of providers.

```go
client := goaikit.NewClient(
    // ... OpenRouter config ...
)

// This call will only use models from Anthropic or Google
output, err := goaikit.Ask[string](context.Background(), client,
    goaikit.WithPrompt("Tell me a joke."),
    goaikit.WithModel("best"), // Use a routing model like "best"
    goaikit.WithOpenRouterProviders("anthropic", "google"),
)
```

**Advanced File Parsing**

By default, file content is extracted by the model itself. With OpenRouter, you can specify a dedicated OCR engine for better results, especially with images.

```go
// Read image bytes
imageBytes, _ := os.ReadFile("image.png")

// Ask a question about the image using Mistral's OCR engine
analysis, err := goaikit.Ask[string](context.Background(), client,
    goaikit.WithPrompt("What text is in this image?"),
    goaikit.WithFile(goaikit.FileImage("image/png", imageBytes)),
    goaikit.WithOpenRouterFileParser(goaikit.ParserEngineMistralOCR),
)
```

### Langfuse Plugin for Observability

Integrate with [Langfuse](https://langfuse.com/) to trace and debug your LLM calls.

**1. Setup**
```go
// Initialize Langfuse client
lf := langfuse.New(context.Background())
defer lf.Flush(context.Background())

// Add the plugin to your goai-kit client
client := goaikit.NewClient(
    goaikit.WithPlugin(goaikit.LangfusePlugin(lf)),
    // ... other options
)
```

**2. Grouping calls in a single trace**

Use `WithTrace` to group multiple `Ask` calls into a single trace. Use `WithGenerationName` to name individual steps (which become Langfuse "Generations"). Each `Ask` call is automatically traced as a generation; `WithTrace` is for grouping them under a single parent trace.

```go
import "github.com/henomis/langfuse-go/model"

goaikit.WithTrace[ResponseType](ctx, client, &model.Trace{Name: "MyMultiStepFlow"}, 
    func(ctx context.Context) (*ResponseType, error) {
        // First call
        step1, err := goaikit.Ask[Step1Output](ctx, client,
            goaikit.WithPrompt("..."),
            goaikit.WithGenerationName("Step1-GetData"),
        )
        // ...
        // Second call
        return goaikit.Ask[ResponseType](ctx, client,
            goaikit.WithPrompt("..."),
            goaikit.WithGenerationName("Step2-Analyze"),
        )
    },
)
```
