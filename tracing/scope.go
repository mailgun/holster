package tracing

import (
	"context"
	"runtime"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Scope state.
type S struct {
	Ctx context.Context
	Span trace.Span
}

type ScopeAction func(s *S) error

// Call action function within a scoped tracing span.
func Scope(ctx context.Context, spanName string, action ScopeAction) error {
	pc, file, line, callerOk := runtime.Caller(1)

	// Determine source file and line number.
	var fileTag string
	if callerOk {
		fileTag = file + ":" + strconv.Itoa(line)
		if spanName == "" {
			spanName = runtime.FuncForPC(pc).Name()
		}
	} else {
		// Rare condition.  Probably a bug in caller.
		fileTag = "unknown"
	}

	// Initialize span.
	ctx, span := globalTracer.Start(ctx, spanName, trace.WithAttributes(
		attribute.String("file", fileTag),
	))
	defer span.End()

	// Call action function.
	return action(&S{
		Ctx: ctx,
		Span: span,
	})
}
