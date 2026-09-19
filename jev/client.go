package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Endpoints. The path is part of the base URL because the two providers do not
// share one: OpenRouter serves the alpha decisions endpoint, TypeSafe its own.
const (
	OpenRouterURL = "https://openrouter.ai/api/alpha/decisions"
	TypeSafeURL   = "https://api.typesafe.ai/v1/systemone"
)

// Model slugs. The slug differs per provider, like the URL.
const (
	OpenRouterModel = "~typesafe/jev-latest"
	TypeSafeModel   = "jev-latest"
)

// Client calls one Jev endpoint. The zero value is not usable; use New. Its fields
// may be changed before the first call, for example to point BaseURL at TypeSafe:
//
//	c := jev.New(key)
//	c.BaseURL, c.Model = jev.TypeSafeURL, jev.TypeSafeModel
//
// HTTP keep-alive is on by default, so a series of calls pays the TLS handshake once.
type Client struct {
	// BaseURL is the full endpoint URL, not just a host.
	BaseURL string
	APIKey  string
	// Model is used by Decide for requests that do not name one themselves.
	Model string
	// HTTP is the client used for the call; replace it to set a timeout or a transport.
	HTTP *http.Client
}

// New returns a client configured for OpenRouter. Set HTTP.Timeout or use a
// context deadline to bound request duration.
func New(apiKey string) *Client {
	return &Client{
		BaseURL: OpenRouterURL,
		APIKey:  apiKey,
		Model:   OpenRouterModel,
		HTTP:    &http.Client{},
	}
}

// Decide asks the questions about state and returns one answer per question, under
// the same keys, using the client's default model.
func (c *Client) Decide(ctx context.Context, state any, questions Questions) (*Response, error) {
	return c.Do(ctx, &Request{Model: c.Model, State: state, Questions: questions})
}

// Do sends req without modifying it. Its Model overrides the client default.
// Configure the client before use; concurrent calls may share an immutable request.
func (c *Client) Do(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, fmt.Errorf("jev: nil request")
	}
	request := *req
	if request.Model == "" {
		request.Model = c.Model
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("jev: encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("jev: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("jev: %w", err)
	}
	defer func() {
		// The body is consumed below; closing it cannot change the result.
		_ = httpResp.Body.Close()
	}()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("jev: read response: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, &APIError{Status: httpResp.StatusCode, Body: string(respBody)}
	}
	var resp Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("jev: decode response: %w", err)
	}
	return &resp, nil
}

// APIError is a non-200 answer from the endpoint.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("jev: http %d: %s", e.Status, truncate([]byte(e.Body), 400))
}

// Retryable reports whether the call is worth repeating after a backoff:
// 429 (rate limited), 529 (overloaded) and the 5xx range. The client does not
// retry by itself.
func (e *APIError) Retryable() bool {
	return e.Status == http.StatusTooManyRequests || (e.Status >= 500 && e.Status < 600)
}

func truncate(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
