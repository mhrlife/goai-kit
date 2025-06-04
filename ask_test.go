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
	"strings"
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
		WithFile(FileImage("image.png", image)),
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
		WithFile(FileImage("file.png", image)),
		WithOpenRouterFileParser(ParserEngineMistralOCR),
	)

	require.NoError(t, err)
	fmt.Println(out.ExactContent)
}

func TestReturnString(t *testing.T) {
	goaiClient := NewClient(
		WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
		WithBaseURL(os.Getenv("OPENROUTER_API_BASE")),
		WithDefaultModel("openai/gpt-4.1-nano"),
		WithLogLevel(slog.LevelDebug),
	)

	out, err := Ask[string](context.Background(), goaiClient,
		WithPrompt("What is the exact content?"),
		WithFile(FileImage("file.png", image)),
		WithOpenRouterFileParser(ParserEngineMistralOCR),
	)

	require.NoError(t, err)
	fmt.Println(*out)
}

//go:embed fixture/fruits.png
var fruitsImage []byte

func TestGeminiSegmentation(t *testing.T) {
	goaiClient := NewClient(
		WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
		WithBaseURL(os.Getenv("OPENROUTER_API_BASE")),
		WithDefaultModel("google/gemini-2.5-flash-preview-05-20"),
		WithLogLevel(slog.LevelDebug),
	)

	out, err := Ask[string](context.Background(), goaiClient,
		WithPrompt("Give the segmentation masks for the Watermelon. Output a JSON list of segmentation masks where each entry contains the 2D bounding box in the key \"box_2d\", the segmentation mask in key \"mask\", and the text label in the key \"label\". Use descriptive labels."),
		WithFile(FileImage("file.png", fruitsImage)),
		WithTemperature(0.0),
		WithRetries(1),
		WithMaxTokens(4096),
		WithOpenRouterFileParser(ParserEngineNative),
	)

	require.NoError(t, err)
	fmt.Println(*out)
}

type CityID struct {
	DisplayName string
	ActualID    string
	AnotherID   string
}

type CityIDSearchArgs struct {
	Query string `jsonschema_description:"The name of the city to search for." json:"query"`
}

type AnotherIDSearchArgs struct {
	Query string `jsonschema_description:"The name of the city to search for." json:"query"`
}

func TestWithTool(t *testing.T) {
	goaiClient := NewClient(
		WithDefaultModel("gpt-4.1-mini"),
		WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
		WithBaseURL(os.Getenv("OPENROUTER_API_BASE")),
		WithLogLevel(slog.LevelDebug),
	)

	out, err := Ask[CityID](context.Background(), goaiClient,
		WithSystem(`You are an agent that must find the city the user is looking for.
You can call the "get_city_id" tool to get the ID and "another" ID of a city based on its names, that is provided in the chat.`),
		WithPrompt("Ads of Jamshideh"),
		WithTool(&Tool[CityIDSearchArgs]{
			Name:        "Get City ID",
			Description: "Get the ID of a city based on its name.",
			Runner: func(ctx *ToolContext, args CityIDSearchArgs) (any, error) {
				require.Equal(t, "jamshideh", strings.ToLower(args.Query))

				return "J-17", nil
			},
		}),
		WithTool(&Tool[AnotherIDSearchArgs]{
			Name:        "Get Another ID",
			Description: "Get another ID of a city based on its name.",
			Runner: func(ctx *ToolContext, args AnotherIDSearchArgs) (any, error) {
				require.Equal(t, "jamshideh", strings.ToLower(args.Query))

				return "ANOTHER-17", nil
			},
		}),
	)

	require.NoError(t, err)
	require.Equal(t, out.ActualID, "J-17")
	require.Equal(t, out.AnotherID, "ANOTHER-17")
}
