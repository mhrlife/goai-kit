package goaikit

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

// Define a simple struct for the expected output
type TestOutput struct {
	Greeting string `json:"greeting"`
	Number   int    `json:"number"`
}

func TestRequestWithActualRequest(t *testing.T) {
	// Create a goaikit Client using the mock OpenAI client
	// Use functional options for configuration
	goaiClient := NewClient(
		WithDefaultModel("openrouter-gemini-2.5-flash-preview"),
		WithLogLevel(slog.LevelDebug), // Example: Set log level to Debug
	)
	// Create AskOptions
	options := AskOptions{
		Prompt: "Say hello and give me a number.",
	}

	out, err := Ask[TestOutput](context.Background(), goaiClient, options)

	require.NoError(t, err)

	fmt.Println(out)
}
