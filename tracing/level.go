package tracing

import (
	"context"

	"github.com/sirupsen/logrus"
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
	logrus.Info("NewLevelTracerProvider()")
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
	if ctxLevel, ok := ctx.Value(logLevelCtxKey).(int64); ok {
		if ctxLevel > t.level {
			return newDummySpan(ctx)
		}
	}

	// Pass-through.
	return t.Tracer.Start(ctx, spanName, opts...)
}
