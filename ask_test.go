package goaikit

import (
	"context"
	_ "embed"
	"fmt"
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

//go:embed fixture/test.pdf
var testPDF []byte

func TestWithFile(t *testing.T) {
	type Output struct {
		PDFContent string `jsonschema_description:"Exact content of the PDF file, with no extra explaination." json:"pdf_content"`
	}

	goaiClient := NewClient(
		WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
		WithBaseURL(os.Getenv("OPENROUTER_API_BASE")),
		WithDefaultModel("google/gemini-2.5-flash-preview-05-20"),
		WithLogLevel(slog.LevelDebug),
	)

	out, err := Ask[Output](context.Background(), goaiClient,
		WithPrompt("What is the content of the file.pdf?"),
		WithFile(FilePDF("file.pdf", testPDF)),
	)

	require.NoError(t, err)
	require.Equal(t, out.PDFContent, "Hello World!")

}

//go:embed fixture/img.png
var image []byte

func TestWithPNG(t *testing.T) {
	type Output struct {
		NumberOfChoices int `jsonschema_description:"Number of choices in the image." json:"number_of_choices"`
	}

	goaiClient := NewClient(
		WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
		WithBaseURL(os.Getenv("OPENROUTER_API_BASE")),
		WithDefaultModel("google/gemini-2.5-flash-preview-05-20"),
		WithLogLevel(slog.LevelDebug),
	)

	out, err := Ask[Output](context.Background(), goaiClient,
		WithPrompt("What is the number of choices in the image?"),
		WithFile(FilePNG("image.png", image)),
	)

	require.NoError(t, err)
	require.Equal(t, out.NumberOfChoices, 4)
}

func TestWithPNGMistralOCR(t *testing.T) {
	type Output struct {
		ExactContent string `jsonschema_description:"Exact content of the PDF, with no extra explanation." json:"exact_content"`
	}

	goaiClient := NewClient(
		WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
		WithBaseURL(os.Getenv("OPENROUTER_API_BASE")),
		WithDefaultModel("openai/gpt-4.1-nano"),
		WithLogLevel(slog.LevelDebug),
	)

	out, err := Ask[Output](context.Background(), goaiClient,
		WithPrompt("What is the exact content?"),
		WithFile(FilePNG("file.png", image)),
		WithOpenRouterFileParser(ParserEngineMistralOCR),
	)

	require.NoError(t, err)
	fmt.Println(out.ExactContent)
}
