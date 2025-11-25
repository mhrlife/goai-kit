package main

import (
	"context"
	"fmt"
	"log"
	"os"

	goaikit "github.com/mhrlife/goai-kit"
	"github.com/mhrlife/goai-kit/tracing"
)

var _ goaikit.ToolExecutor = &AverageNumbersTool{}

type AverageNumbersTool struct {
	goaikit.BaseTool
	Numbers []float64 `json:"numbers" jsonschema:"description=List of numbers to calculate average"`
}

func (t *AverageNumbersTool) AgentToolInfo() goaikit.AgentToolInfo {
	return goaikit.AgentToolInfo{
		Name:        "average_numbers",
		Description: "Calculate the average of a list of numbers.",
	}
}

func (t *AverageNumbersTool) Execute(ctx *goaikit.Context) (any, error) {
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
	// 1. Set up OTEL Langfuse tracer
	tracer, err := tracing.NewOTELLangfuseTracer(tracing.LangfuseConfig{
		SecretKey:   os.Getenv("LLM_COURSE_LANGFUSE_SECRET_KEY"),
		PublicKey:   os.Getenv("LLM_COURSE_LANGFUSE_PUBLIC_KEY"),
		Host:        "cloud.langfuse.com",
		URLPath:     "/api/public/otel/v1/traces",
		Environment: "development",
	})
	if err != nil {
		panic(err)
	}

	// Ensure tracer is flushed before exit
	defer tracer.FlushOrPanic()

	// 2. Create goaikit client
	client := goaikit.NewClient(
		goaikit.WithAPIKey(os.Getenv("LLM_COURSE_OPENROUTER_API_KEY")),
		goaikit.WithBaseURL("https://openrouter.ai/api/v1"),
		goaikit.WithDefaultModel("openai/gpt-4o-mini"),
	)

	// Create agent with tools
	agent := goaikit.CreateAgent(
		client,

		&AverageNumbersTool{},
	).WithCallbacks(goaikit.NewLangfuseCallback(goaikit.LangfuseCallbackConfig{
		Tracer:      tracer.Tracer(),
		ServiceName: "goaikit-simple-agent",
	}))
	fmt.Println("Running agent with tracing...")

	result, err := agent.Invoke(context.Background(), goaikit.InvokeConfig{
		Prompt: "What is the average of the numbers 10, 20, 30, 40, and 50?",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("📊 Final Result:")
	fmt.Println(result)
}
