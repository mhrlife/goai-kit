package goaikit

import (
	"context"
	"log/slog"
)

type Context struct {
	context.Context
	config *AskConfig
	logger *slog.Logger
}

func (c *Context) WithValue(key any, value any) {
	c.Context = context.WithValue(c.Context, key, value)
}
