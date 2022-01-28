// Trace a code block as a scoped span.
// * Use instead of manual instrumentation: `tracer.Start()`/`span.End()`.
// * Must call `InitTracing()` first.
// * Automates start/end of span.
// * Default span name is fully qualified function name.
// * Tags file and line number where span started.
// * If function returned error:
//   * Span is tagged as error.
//   * Sets span attributes `otel.status_code` and `otel.status_description`
//     with error details.
//   * Logs error details to span.

package tracing

import (
	"context"
	"runtime"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type ScopeAction func(ctx context.Context) error

// Scope calls action function within a scoped tracing span.
// Must call `InitTracing()` first.
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
	tracer, ok := ctx.Value(tracerKey{}).(trace.Tracer)
	if !ok {
		// No tracer embedded.  Just call the action function.
		return action(ctx)
	}

	ctx, span := tracer.Start(ctx, spanName, trace.WithAttributes(
		attribute.String("file", fileTag),
	))
	defer span.End()

	// Call action function.
	err := action(ctx)

	// If scope returns an error, mark span with error.
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return err
}
