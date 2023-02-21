package tracing

import (
	"context"
	"fmt"

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
		attribute.String(LogLevelKey, ctxLevel.String()),
	)

	return spanCtx, span
}

func (level Level) String() string {
	if level <= 6 {
		return logLevelNames[level]
	}
	return ""
}

func ParseLogLevel(level string) (Level, error) {
	switch level {
	case "PANIC":
		return PanicLevel, nil
	case "FATAL":
		return FatalLevel, nil
	case "ERROR":
		return ErrorLevel, nil
	case "WARNING":
		return WarnLevel, nil
	case "INFO":
		return InfoLevel, nil
	case "DEBUG":
		return DebugLevel, nil
	case "TRACE":
		return TraceLevel, nil
	default:
		return Level(0), fmt.Errorf("unknown log level %q", level)
	}
}
