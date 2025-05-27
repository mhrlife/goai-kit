package goaikit

import (
	"github.com/henomis/langfuse-go"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"log/slog" // Import slog
	"os"
)

type Client struct {
	client openai.Client
	config Config
	logger *slog.Logger // Add a dedicated logger instance
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Config)

type Config struct {
	ApiKey         string
	ApiBase        string
	RequestOptions []option.RequestOption
	DefaultModel   string
	LogLevel       slog.Level // Add LogLevel field
	BeforeRequest  []BeforeRequestHook
	AfterRequest   []AfterRequestHook

	// Plugin Options

	lf *langfuse.Langfuse
}

// NewClient creates a new goaikit Client with the given options.
func NewClient(opts ...ClientOption) *Client {
	c := Config{
		RequestOptions: make([]option.RequestOption, 0),
		LogLevel:       slog.LevelError,
	}

	// Apply environment variables as initial defaults if options are not provided
	if os.Getenv("OPENAI_API_BASE") != "" {
		c.ApiBase = os.Getenv("OPENAI_API_BASE")
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		c.ApiKey = os.Getenv("OPENAI_API_KEY")
	}

	// Apply functional options, which can override environment variables
	for _, opt := range opts {
		opt(&c)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: c.LogLevel,
	}))

	// Add API Key and Base URL from config to RequestOptions if they are set
	// These are added *after* user-provided RequestOptions via WithRequestOptions
	// so user options take precedence if there's a conflict (e.g., multiple base URLs)
	if c.ApiKey != "" {
		c.RequestOptions = append(c.RequestOptions, option.WithAPIKey(c.ApiKey))
	}
	if c.ApiBase != "" {
		c.RequestOptions = append(c.RequestOptions, option.WithBaseURL(c.ApiBase))
	}

	// Add default middleware (like logging)
	c.RequestOptions = append(
		c.RequestOptions,
		option.WithMiddleware(LoggingMiddleware(logger, c.LogLevel)),
	)

	return &Client{
		client: openai.NewClient(c.RequestOptions...),
		config: c,
		logger: logger, // Assign the dedicated logger
	}
}

// applyBeforeRequestHooks applies the BeforeRequest hooks.
// It returns the modified parameters after applying all hooks.
func (c *Client) applyBeforeRequestHooks(ctx *Context, params openai.ChatCompletionNewParams) openai.ChatCompletionNewParams {
	modifiedParams := params
	for _, hook := range c.config.BeforeRequest {
		// The hook returns a modified ChatCompletionNewParams
		modifiedParams = hook(ctx, modifiedParams)
	}
	return modifiedParams
}

// applyAfterRequestHooks applies the AfterRequest hooks.
// It returns the modified response and error after applying all hooks.
func (c *Client) applyAfterRequestHooks(ctx *Context, response *openai.ChatCompletion, err error) (*openai.ChatCompletion, error) {
	modifiedResponse := response
	modifiedErr := err
	for _, hook := range c.config.AfterRequest {
		modifiedResponse, modifiedErr = hook(ctx, modifiedResponse, modifiedErr)
	}
	return modifiedResponse, modifiedErr
}
