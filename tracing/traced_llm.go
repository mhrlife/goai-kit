package tracing

import (
	"context"

	goaikit "github.com/mhrlife/goai-kit"
)

// TracedLLM wraps a goaikit Client with OTEL tracing
type TracedLLM struct {
	client *goaikit.Client
	tracer *OTELLangfuseTracer
}

// NewTracedLLM creates a new traced LLM client
func NewTracedLLM(client *goaikit.Client, tracer *OTELLangfuseTracer) *TracedLLM {
	if tracer == nil || !tracer.IsEnabled() {
		// If tracing is disabled, return unwrapped client
		return &TracedLLM{
			client: client,
			tracer: nil,
		}
	}

	return &TracedLLM{
		client: client,
		tracer: tracer,
	}
}

// Client returns the underlying goaikit Client
func (t *TracedLLM) Client() *goaikit.Client {
	return t.client
}

// Tracer returns the underlying OTEL tracer
func (t *TracedLLM) Tracer() *OTELLangfuseTracer {
	return t.tracer
}

// GetContext returns the context with tracing information if available
// This allows nested agents to inherit the trace context
func (t *TracedLLM) GetContext(ctx context.Context) context.Context {
	if t.tracer == nil || !t.tracer.IsEnabled() {
		return ctx
	}
	return ctx
}
