// Trace a code block as a scoped span.
// * Use instead of manual instrumentation: `tracer.Start()`/`span.End()`.
// * Must call `InitTracing()` first.
// * Automates start/end of span.
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

	"github.com/mailgun/holster/v4/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type ScopeAction func(ctx context.Context) error

const (
	ErrorClassKey = "error.class"
	ErrorTypeKey  = "error.type"
)

// Start a scope with span named after fully qualified caller function.
func StartScope(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	level := InfoLevel
	spanName, fileTag := getCallerSpanName(2)
	return startSpan(ctx, spanName, fileTag, level, opts...)
}

// Start a scope with span named after fully qualified caller function with
// debug log level.
func StartScopeDebug(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	level := DebugLevel
	spanName, fileTag := getCallerSpanName(2)
	return startSpan(ctx, spanName, fileTag, level, opts...)
}

// Start a scope with span named after fully qualified caller function with
// info log level.
func StartScopeInfo(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	level := InfoLevel
	spanName, fileTag := getCallerSpanName(2)
	return startSpan(ctx, spanName, fileTag, level, opts...)
}

// Start a scope with span named after fully qualified caller function with
// warning log level.
func StartScopeWarn(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	level := WarnLevel
	spanName, fileTag := getCallerSpanName(2)
	return startSpan(ctx, spanName, fileTag, level, opts...)
}

// Start a scope with span named after fully qualified caller function with
// error log level.
func StartScopeError(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	level := ErrorLevel
	spanName, fileTag := getCallerSpanName(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	trace.SpanFromContext(ctx).SetAttributes(attribute.Bool("error", true))
	return ctx
}

// Start a scope with user-provided span name.
func StartNamedScope(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	level := InfoLevel
	fileTag := getFileTag(2)
	return startSpan(ctx, spanName, fileTag, level, opts...)
}

// Start a scope with user-provided span name with debug log level.
func StartNamedScopeDebug(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	level := DebugLevel
	fileTag := getFileTag(2)
	return startSpan(ctx, spanName, fileTag, level, opts...)
}

// Start a scope with user-provided span name with info log level.
func StartNamedScopeInfo(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	level := InfoLevel
	fileTag := getFileTag(2)
	return startSpan(ctx, spanName, fileTag, level, opts...)
}

// Start a scope with user-provided span name with warning log level.
func StartNamedScopeWarn(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	level := WarnLevel
	fileTag := getFileTag(2)
	return startSpan(ctx, spanName, fileTag, level, opts...)
}

// Start a scope with user-provided span name with error log level.
func StartNamedScopeError(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	level := ErrorLevel
	fileTag := getFileTag(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	trace.SpanFromContext(ctx).SetAttributes(attribute.Bool("error", true))
	return ctx
}

// Branch an existing scope with span named after fully qualified caller function.
func BranchScope(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return StartScope(ctx, opts...)
}

// Branch an existing scope with span named after fully qualified caller function with
// debug log level.
func BranchScopeDebug(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return StartScopeDebug(ctx, opts...)
}

// Branch an existing scope with span named after fully qualified caller function with
// info log level.
func BranchScopeInfo(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return StartScopeInfo(ctx, opts...)
}

// Branch an existing scope with span named after fully qualified caller function with
// warn log level.
func BranchScopeWarn(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return StartScopeWarn(ctx, opts...)
}

// Branch an existing scope with span named after fully qualified caller function with
// error log level.
func BranchScopeError(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return StartScopeError(ctx, opts...)
}

// Branch an existing scope with user-provided span name.
func BranchNamedScope(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return StartNamedScope(ctx, spanName, opts...)
}

// Branch an existing scope with user-provided span name with debug log level.
func BranchNamedScopeDebug(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return StartNamedScopeDebug(ctx, spanName, opts...)
}

// Branch an existing scope with user-provided span name with info log level.
func BranchNamedScopeInfo(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return StartNamedScopeInfo(ctx, spanName, opts...)
}

// Branch an existing scope with user-provided span name with warn log level.
func BranchNamedScopeWarn(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return StartNamedScopeWarn(ctx, spanName, opts...)
}

// Branch an existing scope with user-provided span name with error log level.
func BranchNamedScopeError(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return StartNamedScopeError(ctx, spanName, opts...)
}

// End scope created by `StartScope()`/`StartNamedScope()`.
// Logs error return value and ends span.
func EndScope(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)

	// If scope returns an error, mark span with error.
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		if typedErr, ok := err.(*errors.TypedError); ok {
			span.SetAttributes(
				attribute.String(ErrorClassKey, typedErr.Class()),
				attribute.String(ErrorTypeKey, typedErr.Type()),
			)
		}
	}

	span.End()
}

var (
	// Deprecated: Use CallScope
	Scope = CallScope
	// Deprecated: Use CallScopeDebug
	ScopeDebug = CallScopeDebug
	// Deprecated: Use CallScopeInfo
	ScopeInfo = CallScopeInfo
	// Deprecated: Use CallScopeWarn
	ScopeWarn = CallScopeWarn
	// Deprecated: Use CallScopeError
	ScopeError = CallScopeError
	// Deprecated: Use CallScope
	NamedScope = CallNamedScope
	// Deprecated: Use CallNamedScopeDebug
	NamedScopeDebug = CallNamedScopeDebug
	// Deprecated: Use CallNamedScopeInfo
	NamedScopeInfo = CallNamedScopeInfo
	// Deprecated: Use CallNamedScopeWarn
	NamedScopeWarn = CallNamedScopeWarn
	// Deprecated: Use CallNamedScopeError
	NamedScopeError = CallNamedScopeError
)

// CallScope calls action function within a tracing span named after the calling
// function.
// Equivalent to wrapping a code block with `StartScope()`/`EndScope()`.
func CallScope(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	level := InfoLevel
	spanName, fileTag := getCallerSpanName(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeDebug calls action function within a tracing span named after the calling
// function.  Scope tagged with log level debug.
// Equivalent to wrapping a code block with `StartScope()`/`EndScope()`.
func CallScopeDebug(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	level := DebugLevel
	spanName, fileTag := getCallerSpanName(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeInfo calls action function within a tracing span named after the calling
// function.  Scope tagged with log level info.
// Equivalent to wrapping a code block with `StartScope()`/`EndScope()`.
func CallScopeInfo(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	level := InfoLevel
	spanName, fileTag := getCallerSpanName(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeWarn calls action function within a tracing span named after the calling
// function.  Scope tagged with log level warning.
// Equivalent to wrapping a code block with `StartScope()`/`EndScope()`.
func CallScopeWarn(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	level := WarnLevel
	spanName, fileTag := getCallerSpanName(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeError calls action function within a tracing span named after the calling
// function.  Scope tagged with log level error.
// Equivalent to wrapping a code block with `StartScope()`/`EndScope()`.
func CallScopeError(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	level := ErrorLevel
	spanName, fileTag := getCallerSpanName(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	trace.SpanFromContext(ctx).SetAttributes(attribute.Bool("error", true))
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScope calls action function within a tracing span.
// Equivalent to wrapping a code block with `StartNamedScope()`/`EndScope()`.
func CallNamedScope(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	level := InfoLevel
	fileTag := getFileTag(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeDebug calls action function within a tracing span.  Scope tagged
// with log level debug.
// Equivalent to wrapping a code block with `StartNamedScope()`/`EndScope()`.
func CallNamedScopeDebug(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	level := DebugLevel
	fileTag := getFileTag(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeInfo calls action function within a tracing span.  Scope tagged
// with log level info.
// Equivalent to wrapping a code block with `StartNamedScope()`/`EndScope()`.
func CallNamedScopeInfo(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	level := InfoLevel
	fileTag := getFileTag(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeWarn calls action function within a tracing span.  Scope tagged
// with log level warning.
// Equivalent to wrapping a code block with `StartNamedScope()`/`EndScope()`.
func CallNamedScopeWarn(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	level := WarnLevel
	fileTag := getFileTag(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeError calls action function within a tracing span.  Scope tagged
// with log level error.
// Equivalent to wrapping a code block with `StartNamedScope()`/`EndScope()`.
func CallNamedScopeError(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	level := ErrorLevel
	fileTag := getFileTag(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	trace.SpanFromContext(ctx).SetAttributes(attribute.Bool("error", true))
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeBranch calls action function within a tracing span named after the
// calling function.
// Equivalent to wrapping a code block with `BranchScope()`/`EndScope()`.
func CallScopeBranch(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return action(ctx)
	}
	level := InfoLevel
	spanName, fileTag := getCallerSpanName(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeBranchDebug calls action function within a tracing span named after the
// calling function.  Scope tagged with log level debug.
// Equivalent to wrapping a code block with `BranchScopeDebug()`/`EndScope()`.
func CallScopeBranchDebug(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return action(ctx)
	}
	level := DebugLevel
	spanName, fileTag := getCallerSpanName(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeBranchInfo calls action function within a tracing span named after the
// calling function.  Scope tagged with log level info.
// Equivalent to wrapping a code block with `BranchScopeInfo()`/`EndScope()`.
func CallScopeBranchInfo(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return action(ctx)
	}
	level := InfoLevel
	spanName, fileTag := getCallerSpanName(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeBranchWarn calls action function within a tracing span named after the
// calling function.  Scope tagged with log level warn.
// Equivalent to wrapping a code block with `BranchScopeWarn()`/`EndScope()`.
func CallScopeBranchWarn(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return action(ctx)
	}
	level := WarnLevel
	spanName, fileTag := getCallerSpanName(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeBranchError calls action function within a tracing span named after the
// calling function.  Scope tagged with log level error.
// Equivalent to wrapping a code block with `BranchScopeError()`/`EndScope()`.
func CallScopeBranchError(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return action(ctx)
	}
	level := ErrorLevel
	spanName, fileTag := getCallerSpanName(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeBranch calls action function within an existing tracing span.
// Equivalent to wrapping a code block with `BranchNamedScope()`/`EndScope()`.
func CallNamedScopeBranch(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return action(ctx)
	}
	level := InfoLevel
	fileTag := getFileTag(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeBranchDebug calls action function within an existing tracing span.
// Scope tagged with log level debug.
// Equivalent to wrapping a code block with `BranchNamedScope()`/`EndScope()`.
func CallNamedScopeBranchDebug(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return action(ctx)
	}
	level := DebugLevel
	fileTag := getFileTag(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeBranchInfo calls action function within an existing tracing span.
// Scope tagged with log level debug.
// Equivalent to wrapping a code block with `BranchNamedScope()`/`EndScope()`.
func CallNamedScopeBranchInfo(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return action(ctx)
	}
	level := InfoLevel
	fileTag := getFileTag(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeBranchWarn calls action function within an existing tracing span.
// Scope tagged with log level debug.
// Equivalent to wrapping a code block with `BranchNamedScope()`/`EndScope()`.
func CallNamedScopeBranchWarn(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return action(ctx)
	}
	level := WarnLevel
	fileTag := getFileTag(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeBranchError calls action function within an existing tracing span.
// Scope tagged with log level debug.
// Equivalent to wrapping a code block with `BranchNamedScope()`/`EndScope()`.
func CallNamedScopeBranchError(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return action(ctx)
	}
	level := ErrorLevel
	fileTag := getFileTag(2)
	ctx = startSpan(ctx, spanName, fileTag, level, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

func startSpan(ctx context.Context, spanName, fileTag string, level Level, opts ...trace.SpanStartOption) context.Context {
	opts = append(opts, trace.WithAttributes(
		attribute.String("file", fileTag),
	))

	// Embed log level parameter as context value.
	ctx = context.WithValue(ctx, logLevelCtxKey, level)
	ctx, _ = Tracer().Start(ctx, spanName, opts...)
	return ctx
}

func getCallerSpanName(skip int) (spanName, fileTag string) {
	pc, file, line, ok := runtime.Caller(skip)

	// Determine source file and line number.
	if ok {
		fileTag = file + ":" + strconv.Itoa(line)
		spanName = runtime.FuncForPC(pc).Name()
	} else {
		// Rare condition.  Probably a bug in caller.
		fileTag = "unknown"
	}
	return
}

func getFileTag(skip int) string {
	_, file, line, ok := runtime.Caller(skip)

	// Determine source file and line number.
	if !ok {
		// Rare condition.  Probably a bug in caller.
		return "unknown"
	}

	return file + ":" + strconv.Itoa(line)
}
