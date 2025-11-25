package main

import (
	"context"
	"fmt"
	"os"

	goaikit "github.com/mhrlife/goai-kit"
	"github.com/mhrlife/goai-kit/tracing"
)

func main() {
	ctx := context.Background()

	// Set up OTEL Langfuse tracer
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
	defer func() {
		if err := tracer.Flush(); err != nil {
			panic(err)
		}
	}()

	// Create goaikit client
	client := goaikit.NewClient(
		goaikit.WithAPIKey(os.Getenv("LLM_COURSE_OPENROUTER_API_KEY")),
		goaikit.WithBaseURL("https://openrouter.ai/api/v1"),
		goaikit.WithDefaultModel("gpt-5-nano"),
	)

	// Wrap the client with tracing
	tracedLLM := tracing.NewTracedLLM(client, tracer)

	// Create Langfuse callback using OTEL tracer
	langfuseCallback := goaikit.NewLangfuseCallback(goaikit.LangfuseCallbackConfig{
		Tracer:      tracer.Tracer(),
		ServiceName: "goaikit-example",
	})

	// Create an agent with the traced LLM and callback
	statisticAgent := goaikit.CreateAgent(tracedLLM.Client()).
		WithModel("gpt-5-nano").
		WithCallbacks(langfuseCallback)

	// Run the agent
	result, err := statisticAgent.InvokeSimple(ctx, "سلام")
	if err != nil {
		panic(err)
	}

	fmt.Println("Result:", result)

	// Print trace URL
	traceURL := langfuseCallback.GetTraceURL("")
	if traceURL != "" {
		fmt.Println("View trace at:", traceURL)
	}
}
