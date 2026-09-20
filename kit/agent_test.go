package kit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go"
)

// fakeOpenAI serves one canned assistant message per request and records the
// request bodies it saw, so a test can assert on what the agent asked for.
func fakeOpenAI(t *testing.T, contents ...string) (*Client, *[]map[string]any) {
	t.Helper()
	var seen []map[string]any
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		seen = append(seen, body)

		content := contents[min(call, len(contents)-1)]
		call++
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(w, `{
			"id": "chatcmpl-test", "object": "chat.completion", "model": "test-model",
			"choices": [{"index": 0, "finish_reason": "stop",
				"message": {"role": "assistant", "content": %s}}],
			"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`, mustJSON(content)); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)

	return NewClient(
		WithAPIKey("test-key"),
		WithBaseURL(server.URL),
		WithDefaultModel("test-model"),
	), &seen
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

type review struct {
	Verdict string `json:"verdict"`
	Score   int    `json:"score"`
}

// One agent, two output types. Before generic methods the output type lived on
// Agent[Output], so this needed two agents.
func TestAgentInvokeReusesOneAgentForManyOutputTypes(t *testing.T) {
	client, seen := fakeOpenAI(t, "plain text answer", `{"verdict":"ship it","score":9}`)
	agent := client.Agent().WithModel("test-model")

	text, err := agent.Ask(context.Background(), "say something")
	if err != nil {
		t.Fatal(err)
	}
	if text != "plain text answer" {
		t.Fatalf("Ask = %q", text)
	}

	got, err := agent.InvokeSimple[review](context.Background(), "review this")
	if err != nil {
		t.Fatal(err)
	}
	if got.Verdict != "ship it" || got.Score != 9 {
		t.Fatalf("InvokeSimple[review] = %#v", got)
	}

	if len(*seen) != 2 {
		t.Fatalf("want 2 requests, got %d", len(*seen))
	}
	// A string output must not ask the model for a schema; a struct output must.
	if _, ok := (*seen)[0]["response_format"]; ok {
		t.Error("string output sent a response_format")
	}
	format, ok := (*seen)[1]["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("struct output sent no response_format: %v", (*seen)[1])
	}
	if format["type"] != "json_schema" {
		t.Errorf("response_format = %v", format)
	}
}

func TestAgentInvokeRejectsBadConfig(t *testing.T) {
	client, _ := fakeOpenAI(t, "unused")
	agent := client.Agent()

	if _, err := agent.Invoke[string](context.Background(), InvokeConfig{}); err == nil {
		t.Error("accepted a config with neither Prompt nor Messages")
	}
	_, err := agent.Invoke[string](context.Background(), InvokeConfig{
		Prompt:   "a",
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("b")},
	})
	if err == nil {
		t.Error("accepted both Prompt and Messages")
	}
}

func TestAgentInvokeReportsUndecodableOutput(t *testing.T) {
	client, _ := fakeOpenAI(t, "this is not JSON")
	_, err := client.Agent().InvokeSimple[review](context.Background(), "review this")
	if err == nil {
		t.Fatal("accepted a reply that is not the requested schema")
	}
}

func TestClientAgentDefaults(t *testing.T) {
	client, _ := fakeOpenAI(t, "unused")
	agent := client.Agent()
	if agent.Model() != "test-model" {
		t.Errorf("Model() = %q, want the client default", agent.Model())
	}
	if agent.Client() != client {
		t.Error("Client() did not return the client the agent was built from")
	}
	if got := NewAgent(client).Model(); got != agent.Model() {
		t.Errorf("NewAgent disagrees with client.Agent: %q", got)
	}
}
