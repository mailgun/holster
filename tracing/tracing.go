package tracing

import (
	"context"
	"time"

	"github.com/mailgun/holster/v4/errors"
	"github.com/sirupsen/logrus"
	"github.com/uptrace/opentelemetry-go-extra/otellogrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Context Values key for embedding `Tracer` object.
type tracerKey struct{}

var logLevels = []logrus.Level{
	logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel,
	logrus.WarnLevel, logrus.InfoLevel, logrus.DebugLevel,
	logrus.TraceLevel,
}

// InitTracing initializes a global OpenTelemetry tracer provider singleton.
// Call once before using functions in this package.
// Embeds `Tracer` object in returned context.
// Instruments logrus to mirror to active trace.  Must use `WithContext()`
// method.
// Call after initializing logrus.
// libraryName is typically the application's module name.
func InitTracing(ctx context.Context, libraryName string) (context.Context, trace.Tracer, error) {
	exp, err := jaeger.New(jaeger.WithAgentEndpoint())
	if err != nil {
		return ctx, nil, errors.Wrap(err, "error in jaeger.New")
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
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

	return NewTracer(ctx, libraryName)
}

// NewTracer instantiates a new `Tracer` object using global tracer provider.
// Must call `InitTracing()` first.
// Library name is set in span attribute `otel.library.name`.
func NewTracer(ctx context.Context, libraryName string) (context.Context, trace.Tracer, error) {
	tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		return nil, nil, errors.New("OpenTelemetry global tracer provider has not be initialized")
	}

	tracer := tp.Tracer(libraryName)
	ctx = context.WithValue(ctx, tracerKey{}, tracer)
	return ctx, tracer, nil
}

// CloseTracing closes the global OpenTelemetry tracer provider.
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

// TracerFromContext gets embedded `Tracer` from context.
// Returns nil if not found.
func TracerFromContext(ctx context.Context) trace.Tracer {
	tracer, _ := ctx.Value(tracerKey{}).(trace.Tracer)
	return tracer
}
