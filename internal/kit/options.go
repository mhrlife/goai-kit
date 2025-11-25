package kit

import (
	"log/slog"
	"strings"

	"github.com/openai/openai-go/option"
)

// ===== CLIENT OPTIONS ===== //

// WithAPIKey sets the API key for the lfClient.
func WithAPIKey(apiKey string) ClientOption {
	return func(c *Config) {
		c.ApiKey = apiKey
	}
}

// WithBaseURL sets the base URL for the lfClient.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Config) {
		c.ApiBase = strings.TrimSpace(baseURL)
	}
}

// WithDefaultModel sets the default model to use for requests if not specified in AskOptions.
func WithDefaultModel(model string) ClientOption {
	return func(c *Config) {
		c.DefaultModel = model
	}
}

// WithRequestOptions adds additional openai-go request options to the lfClient.
func WithRequestOptions(opts ...option.RequestOption) ClientOption {
	return func(c *Config) {
		c.RequestOptions = append(c.RequestOptions, opts...)
	}
}

// WithLogLevel sets the minimum log level for the lfClient's internal logging.
func WithLogLevel(level slog.Level) ClientOption {
	return func(c *Config) {
		c.LogLevel = level
	}
}
