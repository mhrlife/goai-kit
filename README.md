# GoAI Kit

## Table of Contents

- [GoAI Kit](#goai-kit)
  - [Table of Contents](#table-of-contents)
  - [Installation](#installation)
  - [Usage](#usage)
    - [Simple Example](#simple-example)
    - [Using with Google Gemini (OpenAI-like API)](#using-with-google-gemini-openai-like-api)
    - [Working with Files](#working-with-files)
  - [Plugins](#plugins)
    - [Langfuse Integration](#langfuse-integration)
      - [Setup](#setup)
      - [Enabling the Plugin](#enabling-the-plugin)
      - [Usage Scenarios](#usage-scenarios)
  - [JSON Schema Tags](#json-schema-tags)
  - [Compatibility](#compatibility)

Tired of general AI frameworks trying to do everything, I just wanted a simple way to communicate with LLMs. So, I built this.

This project aims to satisfy the basic needs for interacting with LLMs, focusing on simplicity and direct communication rather than complex abstractions or "magic".

## Installation

To add `goai-kit` to your Go project, run:

```bash
go get github.com/mhrlife/goai-kit
```

## Usage

### Simple Example

Here's a simple example demonstrating how to use `goai-kit` to ask an LLM a question and receive a JSON response:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv" // Optional: for loading .env file
	"github.com/mhrlife/goai-kit"
	"log/slog"
)

// Define a struct for the expected JSON output, using jsonschema tags
type MyOutput struct {
	Message string `json:"message" jsonschema:"description=A greeting message,example=hello"`
	Value   int    `json:"value" jsonschema:"description=An integer value,required"`
}

func main() {
	// Load environment variables (optional)
	godotenv.Load()

	// Create a new client
	// API Key and Base URL can be set via OPENAI_API_KEY and OPENAI_API_BASE env vars
	// or using functional options like goaikit.WithAPIKey("your-api-key")
	client := goaikit.NewClient(
		goaikit.WithDefaultModel("gpt-4o-mini"), // Set a default model
		goaikit.WithLogLevel(slog.LevelDebug),   // Set logging level (optional)
	)

	// Make the Ask request
	output, err := goaikit.Ask[MyOutput](context.Background(), client,
		goaikit.WithPrompt("Generate a JSON object with a 'message' field saying 'hello' and a 'value' field with the number 42."),
		// Example of setting other parameters:
		// goaikit.WithTemperature(0.5),
		// goaikit.WithMaxTokens(50),
	)
	if err != nil {
		log.Fatalf("Error asking LLM: %v", err)
	}

	// Use the unmarshaled output
	fmt.Printf("Received message: %s\n", output.Message)
	fmt.Printf("Received value: %d\n", output.Value)
}
```

### Using with Google Gemini (OpenAI-like API)

You can also use `goai-kit` with other services that provide an OpenAI-compatible API, such as Google Gemini via their OpenAI endpoint.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv" // Optional: for loading .env file
	"github.com/mhrlife/goai-kit"
	"log/slog"
)

// Define a struct for the expected JSON output
type GeminiOutput struct {
	Greeting string `json:"greeting"`
	Number   int    `json:"number"`
}

func main() {
	// Load environment variables (optional)
	godotenv.Load()

	// Create a new client configured for Google Gemini's OpenAI endpoint
	// Ensure GEMINI_API_KEY environment variable is set
	client := goaikit.NewClient(
		goaikit.WithAPIKey(os.Getenv("GEMINI_API_KEY")),
		goaikit.WithDefaultModel("gemini-2.0-flash-001"), // Use a Gemini model
		goaikit.WithBaseURL("https://generativelanguage.googleapis.com/v1beta/openai/"), // Gemini's OpenAI endpoint
		goaikit.WithLogLevel(slog.LevelDebug), // Set logging level (optional)
	)

	// Make the Ask request
	out, err := goaikit.Ask[GeminiOutput](context.Background(), client,
		goaikit.WithPrompt("Say hello and give me a positive, between 10 and 20, number."),
	)
	if err != nil {
		log.Fatalf("Error asking LLM: %v", err)
	}

	// Use the unmarshaled output
	fmt.Printf("Received greeting: %s\n", out.Greeting)
	fmt.Printf("Received number: %d\n", out.Number)
}
```

### Working with Files

You can send files, such as PDFs, along with your prompts. This is useful for tasks like document analysis or summarization.

Use the `goaikit.WithFile()` option and the `goaikit.FilePDF()` helper function to prepare your file.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"io/ioutil" // Required for reading the file

	"github.com/joho/godotenv"
	"github.com/mhrlife/goai-kit"
	"log/slog"
)

// Define a struct for the expected JSON output
type FileAnalysisOutput struct {
	Summary string `json:"summary" jsonschema_description:"A brief summary of the provided PDF content."`
}

func main() {
	godotenv.Load()

	// Read the PDF file content
	pdfBytes, err := ioutil.ReadFile("path/to/your/document.pdf")
	if err != nil {
		log.Fatalf("Failed to read PDF file: %v", err)
	}

	client := goaikit.NewClient(
		// Configure your client (API key, model, etc.)
		// For models that support file inputs, e.g., some OpenRouter models or newer OpenAI models
		goaikit.WithAPIKey(os.Getenv("OPENROUTER_API_KEY")), // Example, use your preferred provider
		goaikit.WithBaseURL(os.Getenv("OPENROUTER_API_BASE")), // Example
		goaikit.WithDefaultModel("google/gemini-2.5-flash-preview-05-20"), // Example model supporting file input
		goaikit.WithLogLevel(slog.LevelDebug),
	)

	// Make the Ask request with the file
	output, err := goaikit.Ask[FileAnalysisOutput](context.Background(), client,
		goaikit.WithPrompt("Summarize the content of the attached PDF file."),
		goaikit.WithFile(goaikit.FilePDF("document.pdf", pdfBytes)),
	)
	if err != nil {
		log.Fatalf("Error asking LLM with file: %v", err)
	}

	fmt.Printf("PDF Summary: %s\n", output.Summary)
}
```
**Note:** Ensure the LLM model you are using supports file inputs (often referred to as multimodal capabilities). The `FilePDF` function creates a data URI for the PDF content.

Remember to set the `OPENAI_API_KEY` environment variable or provide it via `goaikit.WithAPIKey`.

## Plugins

GoAI Kit supports plugins to extend its functionality.

### Langfuse Integration

You can integrate `goai-kit` with [Langfuse](https://langfuse.com/) for observability and tracing of your LLM interactions.

#### Setup

1.  Ensure you have a Langfuse account (cloud or self-hosted).
2.  Set the following environment variables:
    ```bash
    LANGFUSE_SECRET_KEY="your_secret_key"
    LANGFUSE_PUBLIC_KEY="your_public_key"
    LANGFUSE_HOST="https://cloud.langfuse.com" # Or your self-hosted Langfuse URL
    ```

#### Enabling the Plugin

To enable Langfuse integration, initialize the Langfuse client and pass it to `goai-kit` using the `WithPlugin` option:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/henomis/langfuse-go"
	"github.com/joho/godotenv"
	"github.com/mhrlife/goai-kit"
)

type MyOutput struct {
	Message string `json:"message"`
}

func main() {
	godotenv.Load()

	// Initialize Langfuse client
	lf := langfuse.New(context.Background())
	// Ensure to flush Langfuse events before your application exits
	defer lf.Flush(context.Background())

	// Create a new goai-kit client with the Langfuse plugin
	client := goaikit.NewClient(
		goaikit.WithDefaultModel("gpt-4o-mini"),
		goaikit.WithLogLevel(slog.LevelDebug),
		goaikit.WithPlugin(goaikit.LangfusePlugin(lf)), // Enable Langfuse
	)

	// ... rest of your code
}
```

#### Usage Scenarios

**1. Automatic Tracing for Each `Ask` Call**

Once the Langfuse plugin is enabled, every call to `goaikit.Ask` will automatically create a new trace (if one isn't already in the context) and a generation observation in Langfuse.

```go
	// (Assuming client is initialized with Langfuse plugin as shown above)
	output, err := goaikit.Ask[MyOutput](context.Background(), client,
		goaikit.WithPrompt("Tell me a short joke."),
	)
	if err != nil {
		log.Fatalf("Error asking LLM: %v", err)
	}
	fmt.Printf("Joke: %s\n", output.Message)
	// This call will automatically appear as a trace and generation in Langfuse.
```

**2. Grouping Multiple `Ask` Calls under a Single Trace with `WithTrace`**

If you want to group multiple related `Ask` calls under a single Langfuse trace, you can use the `goaikit.WithTrace` function. You can also customize the name of individual LLM calls (generations) within this trace using `goaikit.WithSpanName`.
Note: You'll need to import `github.com/henomis/langfuse-go/model` for `model.Trace`.

```go
	// (Assuming client is initialized with Langfuse plugin as shown above)

	type QuestionResponse struct {
		Answer string `json:"answer"`
	}

	_, err := goaikit.WithTrace[QuestionResponse](context.Background(), client, &model.Trace{Name: "MyCustomTrace"}, func(ctx context.Context) (*QuestionResponse, error) {
		// First call, part of "MyCustomTrace"
		_, err := goaikit.Ask[MyOutput](ctx, client,
			goaikit.WithPrompt("What is the capital of France?"),
			goaikit.WithSpanName("AskCapital"), // Custom name for this generation in Langfuse
		)
		if err != nil {
			return nil, fmt.Errorf("failed to ask capital: %w", err)
		}

		// Second call, also part of "MyCustomTrace"
		return goaikit.Ask[QuestionResponse](ctx, client,
			goaikit.WithPrompt("What is its population?"),
			goaikit.WithSpanName("AskPopulation"), // Custom name for this generation
		)
	})

	if err != nil {
		log.Fatalf("Error in traced operations: %v", err)
	}
	// Both Ask calls will be logged under the "MyCustomTrace" in Langfuse,
	// with individual generations named "AskCapital" and "AskPopulation".
```

## JSON Schema Tags

This package uses `github.com/invopop/jsonschema` to infer the JSON schema from your Go types. You can leverage the various `jsonschema` struct tags (like `description`, `example`, `enum`, etc.) to customize the generated schema. Refer to the `invopop/jsonschema` documentation for available tags and their usage.

## Compatibility

This package is designed to work with **OpenAI-like interfaces**. You can change the base URL to point to compatible APIs (like OpenRouter, etc.) by setting the `OPENAI_API_BASE` environment variable or using the `goaikit.WithBaseURL` functional option when creating the client.
