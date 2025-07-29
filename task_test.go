package goaikit

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/henomis/langfuse-go"
	"os"
	"testing"
)

func TestDeepResearch(t *testing.T) {
	ctx := context.Background()

	langfuseClient := langfuse.New(ctx)
	defer langfuseClient.Flush(ctx)

	client := NewClient(
		WithBaseURL(os.Getenv("OPENAI_API_BASE")),
		WithAPIKey(os.Getenv("OPENAI_API_KEY")),
		WithDefaultModel("o4-mini-deep-research"),
		WithPlugin(LangfusePlugin(langfuseClient)),
	)

	result, err := Task[string](
		ctx,
		client,
		TaskConfig{
			Instructions: "you *MUST* only use the mcp to answer the question.",
			Prompt:       "What is the capital of France in your random world?",
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("==========================")
	fmt.Println(jsonMarshal(result.Output))
}

func jsonMarshal(a any) string {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		panic(fmt.Errorf("failed to marshal JSON: %w", err))
	}
	return string(data)
}
