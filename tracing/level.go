package tracing

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// LevelTracerProvider wraps a TracerProvider to apply log level processing.
// Tag spans with `log.level=n`, where `n` is numeric log level 0-7 (RFC5424).
// If span log level is lower severity than threshold, create a `DummySpan`
// instead.
// `DummySpan` behaves like an alias of its next non-dummy ancestor, but get
// filtered out because does not get exported.  Nested spans containing a mix
// of real and `DummySpan` will be linked as if the `DummySpan` never happened.
type LevelTracerProvider struct {
	*sdktrace.TracerProvider
	level int64
}

// LevelTracer is created by `LevelTracerProvider`.
type LevelTracer struct {
	trace.Tracer
	level int64
}

// LogLevelKey is the span attribute key for storing numeric log level.
const LogLevelKey = "log.level"

var logLevelCtxKey struct{}

func NewLevelTracerProvider(level int64, opts ...sdktrace.TracerProviderOption) *LevelTracerProvider {
	tp := sdktrace.NewTracerProvider(opts...)

	return &LevelTracerProvider{
		TracerProvider: tp,
		level:          level,
	}
}

func (tp *LevelTracerProvider) Tracer(libraryName string, opts ...trace.TracerOption) trace.Tracer {
	tracer := tp.TracerProvider.Tracer(libraryName, opts...)
	return &LevelTracer{
		Tracer: tracer,
		level:  tp.level,
	}
}

func (t *LevelTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	// Check log level.
	ctxLevel, hasLevel := ctx.Value(logLevelCtxKey).(int64)
	if hasLevel {
		// Prevent log level parameter from propagating to child spans.
		ctx = context.WithValue(ctx, logLevelCtxKey, nil)

		if ctxLevel > t.level {
			return newDummySpan(ctx)
		}
	} else {
		ctxLevel = int64(logrus.InfoLevel)
	}

	// Pass-through.
	spanCtx, span := t.Tracer.Start(ctx, spanName, opts...)
	span.SetAttributes(attribute.Int64(LogLevelKey, ctxLevel))

	return spanCtx, span
}
