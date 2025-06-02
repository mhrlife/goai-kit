package goaikit

import (
	"fmt"
	"github.com/openai/openai-go/option"
	"log/slog"
)

// ===== ASK OPTIONS ===== //

// WithPrompt sets the prompt for the Ask request.
func WithPrompt(prompt string, formatting ...any) AskOption {
	if len(formatting) > 0 {
		prompt = fmt.Sprintf(prompt, formatting...)
	}

	return func(ac *AskConfig) { ac.Prompt = prompt }
}

func WithSystem(system string, formatting ...any) AskOption {
	if len(formatting) > 0 {
		system = fmt.Sprintf(system, formatting...)
	}

	return func(ac *AskConfig) { ac.System = system }
}

func WithFile(file ...File) AskOption {
	return func(ac *AskConfig) {
		if len(ac.Files) == 0 {
			ac.Files = make([]File, 0)
		}

		ac.Files = append(ac.Files, file...)
	}
}

// WithModel sets the model for the Ask request.
func WithModel(model string) AskOption {
	return func(ac *AskConfig) { ac.Model = model }
}

// WithTemperature sets the temperature for the Ask request.
func WithTemperature(temp float64) AskOption {
	return func(ac *AskConfig) { ac.Temperature = &temp }
}

// WithMaxTokens sets the maximum number of tokens for the Ask request.
func WithMaxTokens(tokens int64) AskOption {
	return func(ac *AskConfig) { ac.MaxTokens = &tokens }
}

// WithFrequencyPenalty sets the frequency penalty for the Ask request.
func WithFrequencyPenalty(fp float64) AskOption {
	return func(ac *AskConfig) { ac.FrequencyPenalty = &fp }
}

// WithPresencePenalty sets the presence penalty for the Ask request.
func WithPresencePenalty(pp float64) AskOption {
	return func(ac *AskConfig) { ac.PresencePenalty = &pp }
}

// WithTopP sets the TopP sampling parameter for the Ask request.
func WithTopP(topP float64) AskOption {
	return func(ac *AskConfig) { ac.TopP = &topP }
}

// WithUser sets the user identifier for the Ask request.
func WithUser(user string) AskOption {
	return func(ac *AskConfig) { ac.User = user }
}

// WithSeed sets the seed for the Ask request for deterministic outputs.
func WithSeed(seed int64) AskOption {
	return func(ac *AskConfig) { ac.Seed = &seed }
}

// WithAskSpecificRequestOptions adds openai-go request options specific to this Ask call.
func WithAskSpecificRequestOptions(opts ...option.RequestOption) AskOption {
	return func(ac *AskConfig) {
		ac.AskSpecificRequestOptions = append(ac.AskSpecificRequestOptions, opts...)
	}
}

// WithRetries sets the number of retries for the Ask request.
func WithRetries(retries uint) AskOption {
	return func(ac *AskConfig) { ac.Retries = retries }
}

func WithExtraFields(fields map[string]any) AskOption {
	return func(config *AskConfig) {
		config.ExtraFields = fields
	}
}

func WithOpenRouterProviders(providers ...string) AskOption {
	return func(config *AskConfig) {
		if config.ExtraFields == nil {
			config.ExtraFields = make(map[string]any)
		}

		config.ExtraFields["provider"] = map[string]any{
			"only": providers,
		}
	}
}

type ParserEngine string

var (
	ParserEngineMistralOCR ParserEngine = "mistral-ocr"
	ParserEngineNative     ParserEngine = "native"
)

func WithOpenRouterFileParser(parser ParserEngine) AskOption {
	return func(config *AskConfig) {
		if config.ExtraFields == nil {
			config.ExtraFields = make(map[string]any)
		}

		config.ExtraFields["plugins"] = []any{
			map[string]any{
				"id": "file-parser",
				"image": map[string]any{
					"engine": parser,
				},
				"pdf": map[string]any{
					"engine": parser,
				},
			},
		}
	}
}

/// ======= CLIENT OPTIONS ======= ///

// WithAPIKey sets the API key for the lfClient.
func WithAPIKey(apiKey string) ClientOption {
	return func(c *Config) {
		c.ApiKey = apiKey
	}
}

// WithBaseURL sets the base URL for the lfClient.
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

func WithBeforeRequestHook(hook BeforeRequestHook) ClientOption {
	return func(c *Config) {
		c.BeforeRequest = append(c.BeforeRequest, hook)
	}
}

func WithAfterRequestHook(hook AfterRequestHook) ClientOption {
	return func(c *Config) {
		c.AfterRequest = append(c.AfterRequest, hook)
	}
}

func WithPlugin(plugin Plugin) ClientOption {
	return func(c *Config) {
		options := plugin()
		if options == nil {
			return
		}

		for _, opt := range options {
			opt(c)
		}
	}
}
