// OpenTelemetry tools for tracing to Jaeger Tracing.
//
// Why OpenTelemetry?
// Current standard for distributed tracing clients.
// No longer need implementation specific client module (e.g. Jaeger client).
// OpenTracing is now deprecated, superceded by OpenTelemetry.
// See: opentelemetry.io
//
// OpenTelemetry dev reference:
// https://pkg.go.dev/go.opentelemetry.io/otel
//
// Configuration via environment variables:
// https://github.com/open-telemetry/opentelemetry-go/tree/main/exporters/jaeger
// OTEL_EXPORTER_JAEGER_AGENT_HOST=<hostname|ip>
//
// Instruments logrus.
// Must use `WithContext()` method to propagate active trace.
// Logs will mirror to active trace.
// If it's an error level or higher, will mark span as error and set attributes
// `otel.status_code` and `otel.status_description` with the error message.
//
// Find OpenTelemetry instrumentation for various technologies:
// https://opentelemetry.io/registry/?language=go&component=instrumentation

package tracing

import (
	"context"
	"time"

	"github.com/mailgun/holster/v4/errors"
	"github.com/sirupsen/logrus"
	"github.com/uptrace/opentelemetry-go-extra/otellogrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

// Context Values key for embedding tracer object.
type tracerKey struct{}

var logLevels = []logrus.Level{
	logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel,
	logrus.WarnLevel, logrus.InfoLevel, logrus.DebugLevel,
	logrus.TraceLevel,
}

// Initialize an OpenTelemetry global tracer provider.
// Embeds tracer object in returned context.
// Instruments logrus to mirror to active trace.  Must use `WithContext()`
// method.
// Call after initializing logrus.
func InitTracing(ctx context.Context, serviceName string) (context.Context, trace.Tracer, error) {
	exp, err := jaeger.New(jaeger.WithAgentEndpoint())
	if err != nil {
		return ctx, nil, errors.Wrap(err, "error in jaeger.New")
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			// Service name appears in Jaeger "Service" dropdown.
			semconv.ServiceNameKey.String(serviceName),
		)),
	)
	otel.SetTracerProvider(tp)

	// Setup logrus instrumentation.
	// Using logrus.WithContext() will mirror log to embedded span.
	// Using WithFields() also converts to log attributes.
	logLevel := logrus.GetLevel()
	useLevels := []logrus.Level{}
	for _, l := range logLevels {
		if l <= logLevel {
			useLevels = append(useLevels, l)
		}
	}

	logrus.AddHook(otellogrus.NewHook(
		otellogrus.WithLevels(useLevels...),
	))

	tracer := tp.Tracer(serviceName)
	ctx = context.WithValue(ctx, tracerKey{}, tracer)

	return ctx, tracer, nil
}

// Close OpenTelemetry global tracer provider.
// This allows queued up traces to be flushed.
func CloseTracing(ctx context.Context) error {
	tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		return errors.New("OpenTelemetry global tracer provider has not be initialized")
	}

	ctx, cancel := context.WithTimeout(ctx, 5 * time.Second)
	defer cancel()
	err := tp.Shutdown(ctx)
	if err != nil {
		return errors.Wrap(err, "error in tp.Shutdown")
	}

	return nil
}
