package kit

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/openai/openai-go/option"
)

// LoggingMiddleware creates a middleware function that logs OpenAI API requests and responses.
func LoggingMiddleware(logger *slog.Logger, level slog.Level) option.Middleware {
	return func(request *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		// Use the provided logger if the configured log level is sufficient
		if logger.Enabled(request.Context(), level) {
			logger.Debug("OpenAI Request",
				slog.String("method", request.Method),
				slog.String("url", request.URL.String()),
			)

			if request.Body != nil {
				bodyBytes, err := io.ReadAll(request.Body)
				if err != nil {
					logger.Error("Failed to read request body for logging", "error", err)
					// Continue without logging body
				} else {
					// Limit body logging to prevent flooding console with large requests
					bodyString := string(bodyBytes)
					if len(bodyString) > 1024 { // Log first 1KB
						bodyString = bodyString[:1024] + "..."
					}
					logger.Debug("OpenAI Request Body", slog.String("body", bodyString))
					request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Reset the body
				}
			}
		}

		resp, err := next(request)
		if err != nil {
			// Log errors at error level regardless of configured level
			logger.Error("OpenAI Request Failed",
				slog.String("method", request.Method),
				slog.String("url", request.URL.String()),
				slog.String("error", err.Error()),
			)
			return nil, err
		}

		if logger.Enabled(request.Context(), level) {
			logger.Debug("OpenAI Response",
				slog.String("status", resp.Status),
			)

			// log the response body
			if resp.Body != nil {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					logger.Error("Failed to read response body for logging", "error", err)
					// Continue without logging body
				} else {
					// Limit body logging
					bodyString := string(bodyBytes)
					if len(bodyString) > 1024 { // Log first 1KB
						bodyString = bodyString[:1024] + "..."
					}
					logger.Debug("OpenAI Response Body", slog.String("body", strings.TrimSpace(bodyString)))
					resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				}
			}
		}

		return resp, nil
	}
}
