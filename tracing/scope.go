package tracing

import (
	"context"
	"runtime"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Scope state.
type S struct {
	Ctx context.Context
	Span trace.Span
}

type ScopeAction func(s *S) error

// Call action function within a scoped tracing span.
// Decorate span with file and line number where scope started.
// Decorate span with error if one is returned from action function.
// Pass scoped context and span into action function.
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
	err := action(&S{
		Ctx: ctx,
		Span: span,
	})

	// If scope returns an error, mark span with error.
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return err
}
