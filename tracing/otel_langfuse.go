package tracing

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// LangfuseConfig contains configuration for Langfuse OTEL tracing
type LangfuseConfig struct {
	// SecretKey is the Langfuse secret key
	SecretKey string

	// PublicKey is the Langfuse public key
	PublicKey string

	// Host is the Langfuse host URL
	Host    string
	URLPath string

	// Environment is the deployment environment (e.g., "development", "production")
	Environment string

	// ServiceName is the name of the service (optional, defaults to "goaikit")
	ServiceName string

	// ServiceVersion is the version of the service (optional)
	ServiceVersion string
}

// OTELLangfuseTracer wraps the OpenTelemetry tracer provider for Langfuse
type OTELLangfuseTracer struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	config   LangfuseConfig
}

// NewOTELLangfuseTracer creates a new OTEL tracer configured for Langfuse
func NewOTELLangfuseTracer(config LangfuseConfig) (*OTELLangfuseTracer, error) {
	if config.SecretKey == "" || config.PublicKey == "" || config.Host == "" {
		return nil, fmt.Errorf("SecretKey, PublicKey, and Host are required when tracing is enabled")
	}

	// Set defaults
	serviceName := config.ServiceName
	if serviceName == "" {
		serviceName = "goaikit"
	}

	serviceVersion := config.ServiceVersion
	if serviceVersion == "" {
		serviceVersion = "1.0.0"
	}

	// Create resource with service information
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.DeploymentEnvironment(config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP HTTP exporter for Langfuse
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(config.Host),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": fmt.Sprintf(
				"Basic %s",
				base64.RawURLEncoding.EncodeToString([]byte(
					fmt.Sprintf("%s:%s", config.PublicKey, config.SecretKey),
				)),
			),
		}),
	}
	if config.URLPath != "" {
		opts = append(opts, otlptracehttp.WithURLPath(config.URLPath))
	}
	exporter, err := otlptracehttp.New(
		context.Background(), opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create tracer provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set as global provider
	otel.SetTracerProvider(provider)

	// Create tracer
	tracer := provider.Tracer(serviceName, trace.WithInstrumentationVersion(serviceVersion))

	return &OTELLangfuseTracer{
		provider: provider,
		tracer:   tracer,
		config:   config,
	}, nil
}

// Tracer returns the underlying OpenTelemetry tracer
func (t *OTELLangfuseTracer) Tracer() trace.Tracer {
	return t.tracer
}

// Provider returns the underlying tracer provider
func (t *OTELLangfuseTracer) Provider() *sdktrace.TracerProvider {
	return t.provider
}

// Flush ensures all spans are sent to Langfuse
func (t *OTELLangfuseTracer) Flush() error {
	if t.provider == nil {
		return nil
	}

	ctx := context.Background()
	return t.provider.ForceFlush(ctx)
}

func (t *OTELLangfuseTracer) FlushOrPanic() {
	if err := t.Flush(); err != nil {
		slog.Error("failed to flush tracer", "error", err)
		panic(err)
	}
}

// Shutdown shuts down the tracer provider
func (t *OTELLangfuseTracer) Shutdown() error {
	if t.provider == nil {
		return nil
	}

	ctx := context.Background()
	return t.provider.Shutdown(ctx)
}

// IsEnabled returns whether tracing is enabled
func (t *OTELLangfuseTracer) IsEnabled() bool {
	return t.provider != nil
}
