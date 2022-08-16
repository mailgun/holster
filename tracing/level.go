package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type Level uint64

// LevelTracerProvider wraps a TracerProvider to apply log level processing.
// Tag spans with `log.level` and `log.levelNum=n`, where `n` is numeric log
// level 0-6 (Panic, Fatal, Error, Warn, Info, Debug, Trace).
// If span log level is lower severity than threshold, create a `DummySpan`
// instead.
// `DummySpan` behaves like an alias of its next non-dummy ancestor, but gets
// filtered out and omitted from export.  Nested spans containing a mix of real
// and `DummySpan` will be linked as if the `DummySpan` never happened.
type LevelTracerProvider struct {
	*sdktrace.TracerProvider
	level Level
}

// LevelTracer is created by `LevelTracerProvider`.
type LevelTracer struct {
	trace.Tracer
	level Level
}

// LogLevelKey is the span attribute key for storing numeric log level.
const LogLevelKey = "log.level"
const LogLevelNumKey = "log.levelNum"

const (
	PanicLevel Level = iota
	FatalLevel
	ErrorLevel
	WarnLevel
	InfoLevel
	DebugLevel
	TraceLevel
)

var logLevelCtxKey struct{}
var logLevelNames = []string{
	"PANIC",
	"FATAL",
	"ERROR",
	"WARNING",
	"INFO",
	"DEBUG",
	"TRACE",
}

func NewLevelTracerProvider(level Level, opts ...sdktrace.TracerProviderOption) *LevelTracerProvider {
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
	ctxLevel, hasLevel := ctx.Value(logLevelCtxKey).(Level)
	if hasLevel {
		// Prevent log level parameter from propagating to child spans.
		ctx = context.WithValue(ctx, logLevelCtxKey, nil)

		if ctxLevel > t.level {
			return newDummySpan(ctx)
		}
	} else {
		ctxLevel = InfoLevel
	}

	// Pass-through.
	spanCtx, span := t.Tracer.Start(ctx, spanName, opts...)
	span.SetAttributes(
		attribute.Int64(LogLevelNumKey, int64(ctxLevel)),
		attribute.String(LogLevelKey, logLevelStr(ctxLevel)),
	)

	return spanCtx, span
}

func logLevelStr(level Level) string {
	if level >= 0 && level <= 6 {
		return logLevelNames[level]
	}
	return ""
}
