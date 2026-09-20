package jev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClientDo(t *testing.T) {
	for _, model := range []string{"", "custom-model"} {
		t.Run(model, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/decisions" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing headers")
				}
				var req Request
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					return
				}
				wantModel := model
				if wantModel == "" {
					wantModel = OpenRouterModel
				}
				if req.Model != wantModel || req.State != "Help!" {
					t.Errorf("unexpected request: %+v", req)
				}
				q, ok := req.Questions["urgent"].(NoulQuestion)
				if !ok || q.Instructions != "Urgent?" {
					t.Errorf("unexpected question: %#v", req.Questions)
				}
				if _, err := fmt.Fprint(w, `{"model":"jev-latest","answers":{"urgent":{"type":"noul","noul":0.92}}}`); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client := New(NewOpenRouterClientConfig("test-key", OpenRouterModel))
			client.BaseURL = server.URL + "/decisions"
			client.HTTP = nil // The default transport must work too.
			req := &Request{Model: model, State: "Help!", Questions: Questions{"urgent": NoulQuestion{Instructions: "Urgent?"}}}
			var wg sync.WaitGroup
			for range 4 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					resp, err := client.Do(context.Background(), req)
					if err != nil {
						t.Error(err)
						return
					}
					a, ok := resp.Answers["urgent"].(NoulAnswer)
					if !ok || a.Noul != 0.92 {
						t.Errorf("unexpected answer: %#v", resp.Answers)
					}
				}()
			}
			wg.Wait()
			if req.Model != model {
				t.Fatal("Do mutated caller request")
			}
		})
	}
}

func TestClientErrors(t *testing.T) {
	for _, status := range []int{401, 422, 429, 500, 529, 599, 600} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				if _, err := fmt.Fprint(w, `{"error":"test failure"}`); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client := New(NewOpenRouterClientConfig("test-key", OpenRouterModel))
			client.BaseURL = server.URL
			_, err := client.Decide(context.Background(), "state", Questions{})
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("want APIError, got %v", err)
			}
			if apiErr.Status != status || apiErr.Body != `{"error":"test failure"}` {
				t.Fatalf("unexpected error: %+v", apiErr)
			}
			wantRetry := status == 429 || (status >= 500 && status < 600)
			if apiErr.Retryable() != wantRetry {
				t.Fatalf("wrong retry decision for %d", status)
			}
		})
	}
}

func TestClientInvalidRequests(t *testing.T) {
	client := New(NewOpenRouterClientConfig("test-key", OpenRouterModel))
	if _, err := client.Do(context.Background(), nil); err == nil {
		t.Fatal("accepted nil request")
	}
	if _, err := client.Decide(context.Background(), make(chan int), nil); err == nil {
		t.Fatal("accepted unencodable state")
	}
	client.BaseURL = ":invalid"
	if _, err := client.Decide(context.Background(), "state", nil); err == nil {
		t.Fatal("accepted invalid URL")
	}
}

func TestClientMeasuresLatency(t *testing.T) {
	const delay = 20 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		_, _ = w.Write([]byte(`{"model":"jev-latest","answers":{},"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()
	client := New(NewOpenRouterClientConfig("test-key", OpenRouterModel))
	client.BaseURL = server.URL
	resp, err := client.Decide(context.Background(), "state", Questions{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Latency < delay {
		t.Fatalf("latency %v, want at least %v", resp.Latency, delay)
	}
}

func TestClientCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("sent canceled request") }))
	defer server.Close()
	client := New(NewOpenRouterClientConfig("test-key", OpenRouterModel))
	client.BaseURL = server.URL
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Decide(ctx, "state", Questions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want cancellation, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestClientResponseFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reader  io.Reader
		message string
	}{
		{name: "malformed JSON", reader: strings.NewReader(`not JSON`), message: "decode response"},
		{name: "unknown answer", reader: strings.NewReader(`{"answers":{"x":{"type":"future"}}}`), message: `answer "x"`},
		{name: "read failure", reader: failedReader{}, message: "read response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &trackedBody{Reader: tc.reader}
			client := New(NewOpenRouterClientConfig("test-key", OpenRouterModel))
			client.HTTP = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
			})}
			_, err := client.Decide(context.Background(), "state", Questions{})
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("unexpected error: %v", err)
			}
			if !body.closed {
				t.Fatal("response body not closed")
			}
		})
	}
}
