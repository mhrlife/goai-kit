package goaikit

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"testing"
	"time"
)

// Define a simple struct for the expected output
type TestOutput struct {
	Greeting string `json:"greeting" `
	Number   int    `json:"number"`
}

func TestRequestWithActualRequest(t *testing.T) {
	goaiClient := NewClient(
		WithDefaultModel("gpt-4o-mini"),
		WithLogLevel(slog.LevelDebug),
	)

	out, err := Ask[TestOutput](context.Background(), goaiClient,
		WithPrompt("Say hello and give me a positive, between 10 and 20, number."),
	)
	require.NoError(t, err)
	require.NotZero(t, out.Number)
}

func TestGoogleGeminiOpenAI(t *testing.T) {
	goaiClient := NewClient(
		WithAPIKey(os.Getenv("GEMINI_API_KEY")),
		WithDefaultModel("gemini-2.0-flash-001"),
		WithBaseURL("https://generativelanguage.googleapis.com/v1beta/openai/"),
	)
	out, err := Ask[TestOutput](context.Background(), goaiClient,
		WithPrompt("Say hello and give me a positive, between 10 and 20, number."),
	)

	require.NoError(t, err)
	require.NotZero(t, out.Number)
}

func TestOpenRouterProvider(t *testing.T) {
	goaiClient := NewClient(
		WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
		WithBaseURL(os.Getenv("OPENROUTER_API_BASE")),
		WithDefaultModel("meta-llama/llama-4-scout"),
		WithLogLevel(slog.LevelDebug),
	)

	out, err := Ask[TestOutput](context.Background(), goaiClient,
		WithPrompt("Say hello and give me a positive, between 10 and 20, number. now: %v", time.Now()),
		WithOpenRouterProviders("groq"),
	)

	fmt.Println(out, err)
}
