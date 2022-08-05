package tracing

import (
	"context"

	"github.com/mailgun/holster/v4/errors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type LevelTracer struct {
	trace.Tracer
	level logrus.Level
}

var levelKey struct{}

// NewTracerWithLevel creates a tracer that generates dummy spans for spans
// started with log level above provided threshold.
func NewTracerWithLevel(ctx context.Context, libraryName string, level logrus.Level) (context.Context, trace.Tracer, error) {
	tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		return nil, nil, errors.New("OpenTelemetry global tracer provider has not been initialized")
	}

	tracer := tp.Tracer(libraryName)
	ctx = ContextWithTracer(ctx, tracer)
	lTracer := &LevelTracer{
		Tracer: tracer,
		level:  level,
	}

	return ctx, lTracer, nil
}

func (t *LevelTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	// Check log level.
	if ctxLevel, ok := ctx.Value(levelKey).(logrus.Level); ok {
		if ctxLevel > t.level {
			return newDummySpan(ctx)
		}
	}

	// Pass-through.
	return t.Tracer.Start(ctx, spanName, opts...)
}
