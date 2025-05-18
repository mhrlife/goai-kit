package goaikit

import (
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
}

// NewClient creates a new goaikit Client with the given options.
func NewClient(opts ...ClientOption) *Client {
	// Initialize a default config with a default log level
	c := Config{
		RequestOptions: make([]option.RequestOption, 0),
		LogLevel:       slog.LevelInfo, // Default log level
	}

	// Create a logger instance for the client
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: c.LogLevel,
	}))

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
		option.WithMaxRetries(3),
		option.WithMiddleware(LoggingMiddleware(logger, c.LogLevel)),
	)

	return &Client{
		client: openai.NewClient(c.RequestOptions...),
		config: c,
		logger: logger, // Assign the dedicated logger
	}
}

// WithAPIKey sets the API key for the client.
func WithAPIKey(apiKey string) ClientOption {
	return func(c *Config) {
		c.ApiKey = apiKey
	}
}

// WithBaseURL sets the base URL for the client.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Config) {
		c.ApiBase = baseURL
	}
}

// WithDefaultModel sets the default model to use for requests if not specified in AskOptions.
func WithDefaultModel(model string) ClientOption {
	return func(c *Config) {
		c.DefaultModel = model
	}
}

// WithRequestOptions adds additional openai-go request options to the client.
func WithRequestOptions(opts ...option.RequestOption) ClientOption {
	return func(c *Config) {
		c.RequestOptions = append(c.RequestOptions, opts...)
	}
}

// WithLogLevel sets the minimum log level for the client's internal logging.
func WithLogLevel(level slog.Level) ClientOption {
	return func(c *Config) {
		c.LogLevel = level
	}
}
