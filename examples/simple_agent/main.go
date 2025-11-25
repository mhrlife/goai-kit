package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mhrlife/goai-kit/internal/callback"
	"github.com/mhrlife/goai-kit/internal/kit"
	"github.com/mhrlife/goai-kit/internal/tracing"
)

var _ kit.ToolExecutor = &AverageNumbersTool{}

type AverageNumbersTool struct {
	kit.BaseTool
	Numbers []float64 `json:"numbers" jsonschema:"description=List of numbers to calculate average"`
}

func (t *AverageNumbersTool) AgentToolInfo() kit.AgentToolInfo {
	return kit.AgentToolInfo{
		Name:        "average_numbers",
		Description: "Calculate the average of a list of numbers.",
	}
}

func (t *AverageNumbersTool) Execute(ctx *kit.Context) (any, error) {
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

	// 2. Create kit client
	client := kit.NewClient(
		kit.WithAPIKey(os.Getenv("LLM_COURSE_OPENROUTER_API_KEY")),
		kit.WithBaseURL("https://openrouter.ai/api/v1"),
		kit.WithDefaultModel("openai/gpt-4o-mini"),
	)

	// Create agent with tools
	agent := kit.CreateAgent(
		client,

		&AverageNumbersTool{},
	).WithCallbacks(callback.NewLangfuseCallback(callback.LangfuseCallbackConfig{
		Tracer:      tracer.Tracer(),
		ServiceName: "kit-simple-agent",
	}))
	fmt.Println("Running agent with tracing...")

	result, err := agent.Invoke(context.Background(), kit.InvokeConfig{
		Prompt: "What is the average of the numbers 10, 20, 30, 40, and 50?",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("📊 Final Result:")
	fmt.Println(result)
}
