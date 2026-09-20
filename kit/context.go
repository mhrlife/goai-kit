package kit

import (
	"context"
	"log/slog"
)

// Context is what a ToolExecutor receives: the caller's context plus the client's
// logger, so a tool can log without being handed one separately.
type Context struct {
	context.Context
	logger *slog.Logger
}

// NewContext wraps ctx for a tool call. Agents build this themselves; it is
// exported so other packages (the MCP bridge, tests) can invoke a tool directly.
func NewContext(ctx context.Context, logger *slog.Logger) *Context {
	if logger == nil {
		logger = slog.Default()
	}
	return &Context{Context: ctx, logger: logger}
}

// Logger returns the logger a tool should write to. It is never nil.
func (c *Context) Logger() *slog.Logger {
	if c.logger == nil {
		return slog.Default()
	}
	return c.logger
}

func (c *Context) WithValue(key, value any) {
	c.Context = context.WithValue(c.Context, key, value)
}
