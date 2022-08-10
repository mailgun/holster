package tracing

import (
	"context"

	"github.com/sirupsen/logrus"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type LevelTracerProvider struct {
	*sdktrace.TracerProvider
	level int64
}

type LevelTracer struct {
	trace.Tracer
	level int64
}

const LogLevelKey = "log.level"

var levelKey struct{}

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
	if ctxLevel, ok := ctx.Value(levelKey).(int64); ok {
		if ctxLevel > t.level {
			return newDummySpan(ctx)
		}
	}

	// Pass-through.
	return t.Tracer.Start(ctx, spanName, opts...)
}
