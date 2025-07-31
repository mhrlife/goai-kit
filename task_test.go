package goaikit

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/henomis/langfuse-go"
	"github.com/openai/openai-go/responses"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
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

	type Output struct {
		Found  bool
		Result string
	}
	out, _, err := DeepResearch[Output](
		ctx,
		client,
		TaskConfig{
			Instructions: "you *MUST* only use the mcp to answer the question.",
			Prompt:       "What is the capital of France in your random world?",
			MCPServers: []responses.ToolMcpParam{
				NewApprovedMCPServer(
					"capitals",
					"https://b9e32020e949.ngrok-free.app/default/sse",
				),
			},
		},
	)
	require.NoError(t, err)
	require.Equal(t, true, out.Found)
	require.Equal(t, "tehran", strings.ToLower(out.Result))
}

func TestNormalTask(t *testing.T) {
	ctx := context.Background()

	langfuseClient := langfuse.New(ctx)
	defer langfuseClient.Flush(ctx)

	client := NewClient(
		WithBaseURL(os.Getenv("OPENAI_API_BASE")),
		WithAPIKey(os.Getenv("OPENAI_API_KEY")),
		WithDefaultModel("gpt-4.1-mini"),
		WithPlugin(LangfusePlugin(langfuseClient)),
	)

	type Output struct {
		Found  bool
		Result string
	}
	out, raw, err := DeepResearch[Output](
		ctx,
		client,
		TaskConfig{
			Instructions: "you *MUST* only use the mcp to answer the question.",
			Prompt:       "What is the capital of France in your random world?",
			MCPServers: []responses.ToolMcpParam{
				NewApprovedMCPServer(
					"capitals",
					"https://7006be4cfb29.ngrok-free.app/default/sse",
				),
			},
		},
	)
	require.NoError(t, err)
	require.Equal(t, true, out.Found)
	require.Equal(t, "تهران", strings.ToLower(out.Result))
	fmt.Println(jsonMarshal(raw))
}

func jsonMarshal(a any) string {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		panic(fmt.Errorf("failed to marshal JSON: %w", err))
	}
	return string(data)
}
