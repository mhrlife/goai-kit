package goaikit

import (
	"context"
	"github.com/henomis/langfuse-go"
	"github.com/henomis/langfuse-go/model"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"testing"
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
	type Response struct {
		IsPositive bool
	}

	lf := langfuse.New(context.Background())
	defer lf.Flush(context.Background())

	goaiClient := NewClient(
		WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
		WithBaseURL(os.Getenv("OPENROUTER_API_BASE")),
		WithDefaultModel("openai/gpt-4.1-nano"),
		WithLogLevel(slog.LevelDebug),
		WithPlugin(LangfusePlugin(lf)),
	)

	result, err := WithTrace[Response](context.Background(), goaiClient, &model.Trace{
		Name:  "TestOpenRouterProvider",
		Input: "Is positive?",
	}, func(ctx context.Context) (*Response, error) {
		out, err := Ask[TestOutput](ctx, goaiClient,
			WithPrompt("Say hello and give me a positive, between 10 and 20, number."),
			WithSpanName("Ask for positive number"),
		)
		if err != nil {
			return nil, err
		}

		t.Logf("Response: %+v", out)

		return Ask[Response](
			ctx,
			goaiClient,
			WithPrompt("is %v positive?", out.Number),
			WithSpanName("Check if number is positive"),
		)
	}, WithTraceOutput(func(t *Response) any {
		return t.IsPositive
	}))

	require.NoError(t, err)
	require.True(t, result.IsPositive)
}
