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

Send files (PDFs, images) for multimodal analysis.

```go
// Read image bytes
imageBytes, _ := os.ReadFile("image.png")

type ImageAnalysis struct {
    Description string `json:"description"`
}

// Ask a question about the image
analysis, err := goaikit.Ask[ImageAnalysis](context.Background(), client,
    goaikit.WithPrompt("What is in this image?"),
    goaikit.WithFile(goaikit.FileImage("image.png", imageBytes)),
    goaikit.WithDefaultModel("google/gemini-pro-vision"), // Use a model that supports vision
)
```

### 5. Graphs for Multi-Step Workflows

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

Use `WithTrace` to group multiple `Ask` calls into a single trace. Use `WithSpanName` to name individual steps. Each `Ask` call is automatically traced; `WithTrace` is for grouping them.

```go
import "github.com/henomis/langfuse-go/model"

goaikit.WithTrace[ResponseType](ctx, client, &model.Trace{Name: "MyMultiStepFlow"}, 
    func(ctx context.Context) (*ResponseType, error) {
        // First call
        step1, err := goaikit.Ask[Step1Output](ctx, client,
            goaikit.WithPrompt("..."),
            goaikit.WithSpanName("Step1-GetData"),
        )
        // ...
        // Second call
        return goaikit.Ask[ResponseType](ctx, client,
            goaikit.WithPrompt("..."),
            goaikit.WithSpanName("Step2-Analyze"),
        )
    },
)
```
