# GoaiKit Tracing Package

This package provides OpenTelemetry-based tracing for GoaiKit, with built-in support for Langfuse.

## Features

- OpenTelemetry-based tracing
- Direct integration with Langfuse via OTLP
- Properly handles nested observations and trace IDs
- Context management for child callbacks

## Installation

```bash
go get github.com/mhrlife/goai-kit/tracing
```

## Usage

### Basic Setup

```go
package main

import (
    "context"
    "os"

    "github.com/mhrlife/goai-kit"
    "github.com/mhrlife/goai-kit/tracing"
)

func main() {
    ctx := context.Background()

    // Create OTEL Langfuse tracer
    tracer, err := tracing.NewOTELLangfuseTracer(tracing.LangfuseConfig{
        Enabled:     true,
        SecretKey:   os.Getenv("LANGFUSE_SECRET_KEY"),
        PublicKey:   os.Getenv("LANGFUSE_PUBLIC_KEY"),
        Host:        os.Getenv("LANGFUSE_BASE_URL"), // e.g., "cloud.langfuse.com"
        Environment: "development",
        ServiceName: "my-service", // optional
    })
    if err != nil {
        panic(err)
    }
    defer tracer.Flush()

    // Create goaikit client
    client := goaikit.NewClient(
        goaikit.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
    )

    // Wrap with tracing
    tracedLLM := tracing.NewTracedLLM(client, tracer)

    // Create callback
    callback := goaikit.NewLangfuseCallback(goaikit.LangfuseCallbackConfig{
        Tracer:      tracer.Tracer(),
        ServiceName: "my-service",
    })

    // Create agent with callback
    agent := goaikit.CreateAgent(tracedLLM.Client()).
        WithCallbacks(callback)

    // Run agent
    result, err := agent.InvokeSimple(ctx, "Hello!")
    if err != nil {
        panic(err)
    }

    // Get trace URL
    fmt.Println("Trace URL:", callback.GetTraceURL(""))
}
```

### Configuration Options

#### LangfuseConfig

- `Enabled`: Enable/disable tracing
- `SecretKey`: Langfuse secret key (required)
- `PublicKey`: Langfuse public key (required)
- `Host`: Langfuse host URL (required)
- `Environment`: Deployment environment (e.g., "development", "production")
- `ServiceName`: Service name (optional, defaults to "goaikit")
- `ServiceVersion`: Service version (optional, defaults to "1.0.0")

#### LangfuseCallbackConfig

- `Tracer`: OpenTelemetry tracer (required)
- `ServiceName`: Service name (optional, defaults to "goaikit")
- `TraceID`: Reuse existing trace ID (optional)
- `ParentContext`: Create child callback (optional)

## Trace Hierarchy

The tracing system creates the following hierarchy:

```
Trace (top-level)
└── Agent Run (root span)
    ├── LLM Generation (generation span)
    │   └── usage, model, input/output
    └── Tool Call (tool span)
        └── tool name, input/output
```

## Migration from langfuse-go

If you were using the old `langfuse-go` package:

**Before:**
```go
import "github.com/henomis/langfuse-go"

lfClient := langfuse.New(...)
callback := goaikit.NewLangfuseCallback(goaikit.LangfuseCallbackConfig{
    Client: lfClient,
})
```

**After:**
```go
import "github.com/mhrlife/goai-kit/tracing"

tracer, _ := tracing.NewOTELLangfuseTracer(tracing.LangfuseConfig{
    SecretKey: "...",
    PublicKey: "...",
    Host: "...",
})
defer tracer.Flush()

callback := goaikit.NewLangfuseCallback(goaikit.LangfuseCallbackConfig{
    Tracer: tracer.Tracer(),
})
```

## Notes

- Always call `tracer.Flush()` before your application exits to ensure all spans are sent
- The tracer uses batching for better performance
- Spans are created with proper parent-child relationships
- Context is managed automatically following OpenTelemetry patterns
