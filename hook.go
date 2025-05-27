package goaikit

import (
	"github.com/openai/openai-go"
)

type BeforeRequestHook func(ctx *Context, params openai.ChatCompletionNewParams) openai.ChatCompletionNewParams
type AfterRequestHook func(ctx *Context, response *openai.ChatCompletion, err error) (*openai.ChatCompletion, error)
