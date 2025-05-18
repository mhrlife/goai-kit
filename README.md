# GoAI Kit

Tired of general AI frameworks trying to do everything, I just wanted a simple way to communicate with LLMs. So, I built this.

This project aims to satisfy the basic needs for interacting with LLMs, focusing on simplicity and direct communication rather than complex abstractions or "magic".

## Installation

To add `goai-kit` to your Go project, run:

```bash
go get github.com/mhrlife/goai-kit
```

## Usage

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

	// Define the options for the Ask request
	options := goaikit.AskOptions{
		Prompt: "Generate a JSON object with a 'message' field saying 'hello' and a 'value' field with the number 42.",
	}

	// Make the Ask request
	output, err := goaikit.Ask[MyOutput](context.Background(), client, options)
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

	// Define the options for the Ask request
	options := goaikit.AskOptions{
		Prompt: "Say hello and give me a positive, between 10 and 20, number.",
	}

	// Make the Ask request
	out, err := goaikit.Ask[GeminiOutput](context.Background(), client, options)
	if err != nil {
		log.Fatalf("Error asking LLM: %v", err)
	}

	// Use the unmarshaled output
	fmt.Printf("Received greeting: %s\n", out.Greeting)
	fmt.Printf("Received number: %d\n", out.Number)
}
```

Remember to set the `OPENAI_API_KEY` environment variable or provide it via `goaikit.WithAPIKey`.

## JSON Schema Tags

This package uses `github.com/invopop/jsonschema` to infer the JSON schema from your Go types. You can leverage the various `jsonschema` struct tags (like `description`, `example`, `enum`, etc.) to customize the generated schema. Refer to the `invopop/jsonschema` documentation for available tags and their usage.

## Compatibility

This package is designed to work with **OpenAI-like interfaces**. You can change the base URL to point to compatible APIs (like OpenRouter, etc.) by setting the `OPENAI_API_BASE` environment variable or using the `goaikit.WithBaseURL` functional option when creating the client.
