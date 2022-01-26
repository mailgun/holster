// OpenTelemetry tools for tracing to Jaeger Tracing.
//
// OpenTelemetry dev reference:
// https://pkg.go.dev/go.opentelemetry.io/otel
//
// Configuration via environment variables:
// https://github.com/open-telemetry/opentelemetry-go/tree/main/exporters/jaeger
// OTEL_EXPORTER_JAEGER_AGENT_HOST=<hostname|ip>
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

var globalTracer trace.Tracer

// Initialize an OpenTelemetry global tracer.
// Instrument logrus to mirror to active trace.  Must use `WithContext()`
// method.
func InitTracing(serviceName string) (trace.Tracer, error) {
	exp, err := jaeger.New(jaeger.WithAgentEndpoint())
	if err != nil {
		return nil, errors.Wrap(err, "Error in jaeger.New")
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
	logrus.AddHook(otellogrus.NewHook(
		otellogrus.WithLevels(
			logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel,
			logrus.WarnLevel, logrus.InfoLevel, logrus.DebugLevel,
		),
	))

	globalTracer = tp.Tracer(serviceName)

	return globalTracer, nil
}

// Close OpenTelemetry global tracer.
// This allows queued up traces to be flushed.
func CloseTracing(ctx context.Context) error {
	if globalTracer != nil {
		ctx, cancel := context.WithTimeout(ctx, 5 * time.Second)
		defer cancel()
		tp := otel.GetTracerProvider().(*sdktrace.TracerProvider)
		err := tp.Shutdown(ctx)
		if err != nil {
			return errors.Wrap(err, "Error in globalTracer.Shutdown")
		}
	}

	return nil
}
