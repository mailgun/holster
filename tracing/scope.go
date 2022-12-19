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

// StartScope start a scope with span named after fully qualified caller function.
func StartScope(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	return startScope(ctx, InfoLevel, 1, opts...)
}

// StartScopeDebug start a scope with span named after fully qualified caller function with
// debug log level.
func StartScopeDebug(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	return startScope(ctx, DebugLevel, 1, opts...)
}

// StartScopeInfo start a scope with span named after fully qualified caller function with
// info log level.
func StartScopeInfo(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	return startScope(ctx, InfoLevel, 1, opts...)
}

// StartScopeWarn start a scope with span named after fully qualified caller function with
// warning log level.
func StartScopeWarn(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	return startScope(ctx, WarnLevel, 1, opts...)
}

// StartScopeError start a scope with span named after fully qualified caller function with
// error log level.
func StartScopeError(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	return startScope(ctx, ErrorLevel, 1, opts...)
}

// Start a scope with user-provided span name.
func StartNamedScope(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	return startNamedScope(ctx, spanName, InfoLevel, 1, opts...)
}

// StartNamedScopeDebug start a scope with user-provided span name with debug log level.
func StartNamedScopeDebug(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	return startNamedScope(ctx, spanName, DebugLevel, 1, opts...)
}

// StartNamedScopeInfo start a scope with user-provided span name with info log level.
func StartNamedScopeInfo(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	return startNamedScope(ctx, spanName, InfoLevel, 1, opts...)
}

// StartNamedScopeWarn start a scope with user-provided span name with warning log level.
func StartNamedScopeWarn(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	return startNamedScope(ctx, spanName, WarnLevel, 1, opts...)
}

// StartNamedScopeError start a scope with user-provided span name with error log level.
func StartNamedScopeError(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	return startNamedScope(ctx, spanName, ErrorLevel, 1, opts...)
}

// BranchScope branch an existing scope with span named after fully qualified caller function.
func BranchScope(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return startScope(ctx, InfoLevel, 1, opts...)
}

// BranchScopeDebug branch an existing scope with span named after fully qualified caller function with
// debug log level.
func BranchScopeDebug(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return startScope(ctx, DebugLevel, 1, opts...)
}

// BranchScopeInfo branch an existing scope with span named after fully qualified caller function with
// info log level.
func BranchScopeInfo(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return startScope(ctx, InfoLevel, 1, opts...)
}

// BranchScopeWarn branch an existing scope with span named after fully qualified caller function with
// warn log level.
func BranchScopeWarn(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return startScope(ctx, WarnLevel, 1, opts...)
}

// BranchScopeError branch an existing scope with span named after fully qualified caller function with
// error log level.
func BranchScopeError(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return startScope(ctx, ErrorLevel, 1, opts...)
}

// BranchNamedScope branch an existing scope with user-provided span name.
func BranchNamedScope(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return startNamedScope(ctx, spanName, InfoLevel, 1, opts...)
}

// BranchNamedScopeDebug branch an existing scope with user-provided span name with debug log level.
func BranchNamedScopeDebug(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return startNamedScope(ctx, spanName, DebugLevel, 1, opts...)
}

// BranchNamedScopeInfo branch an existing scope with user-provided span name with info log level.
func BranchNamedScopeInfo(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return startNamedScope(ctx, spanName, InfoLevel, 1, opts...)
}

// BranchNamedScopeWarn branch an existing scope with user-provided span name with warn log level.
func BranchNamedScopeWarn(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return startNamedScope(ctx, spanName, WarnLevel, 1, opts...)
}

// BranchNamedScopeError branch an existing scope with user-provided span name with error log level.
func BranchNamedScopeError(ctx context.Context, spanName string, opts ...trace.SpanStartOption) context.Context {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	return startNamedScope(ctx, spanName, ErrorLevel, 1, opts...)
}

// EndScope end scope created by `StartScope()`/`StartNamedScope()`.
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
	spanName, fileTag := getCallerSpanName(1)
	ctx = startSpan(ctx, spanName, fileTag, InfoLevel, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeDebug calls action function within a tracing span named after the calling
// function.  Scope tagged with log level debug.
// Equivalent to wrapping a code block with `StartScope()`/`EndScope()`.
func CallScopeDebug(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	spanName, fileTag := getCallerSpanName(1)
	ctx = startSpan(ctx, spanName, fileTag, DebugLevel, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeInfo calls action function within a tracing span named after the calling
// function.  Scope tagged with log level info.
// Equivalent to wrapping a code block with `StartScope()`/`EndScope()`.
func CallScopeInfo(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	spanName, fileTag := getCallerSpanName(1)
	ctx = startSpan(ctx, spanName, fileTag, InfoLevel, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeWarn calls action function within a tracing span named after the calling
// function.  Scope tagged with log level warning.
// Equivalent to wrapping a code block with `StartScope()`/`EndScope()`.
func CallScopeWarn(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	spanName, fileTag := getCallerSpanName(1)
	ctx = startSpan(ctx, spanName, fileTag, WarnLevel, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallScopeError calls action function within a tracing span named after the calling
// function.  Scope tagged with log level error.
// Equivalent to wrapping a code block with `StartScope()`/`EndScope()`.
func CallScopeError(ctx context.Context, action ScopeAction, opts ...trace.SpanStartOption) error {
	spanName, fileTag := getCallerSpanName(1)
	ctx = startSpan(ctx, spanName, fileTag, ErrorLevel, opts...)
	trace.SpanFromContext(ctx).SetAttributes(attribute.Bool("error", true))
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScope calls action function within a tracing span.
// Equivalent to wrapping a code block with `StartNamedScope()`/`EndScope()`.
func CallNamedScope(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	fileTag := getFileTag(1)
	ctx = startSpan(ctx, spanName, fileTag, InfoLevel, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeDebug calls action function within a tracing span.  Scope tagged
// with log level debug.
// Equivalent to wrapping a code block with `StartNamedScope()`/`EndScope()`.
func CallNamedScopeDebug(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	fileTag := getFileTag(1)
	ctx = startSpan(ctx, spanName, fileTag, DebugLevel, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeInfo calls action function within a tracing span.  Scope tagged
// with log level info.
// Equivalent to wrapping a code block with `StartNamedScope()`/`EndScope()`.
func CallNamedScopeInfo(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	fileTag := getFileTag(1)
	ctx = startSpan(ctx, spanName, fileTag, InfoLevel, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeWarn calls action function within a tracing span.  Scope tagged
// with log level warning.
// Equivalent to wrapping a code block with `StartNamedScope()`/`EndScope()`.
func CallNamedScopeWarn(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	fileTag := getFileTag(1)
	ctx = startSpan(ctx, spanName, fileTag, WarnLevel, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

// CallNamedScopeError calls action function within a tracing span.  Scope tagged
// with log level error.
// Equivalent to wrapping a code block with `StartNamedScope()`/`EndScope()`.
func CallNamedScopeError(ctx context.Context, spanName string, action ScopeAction, opts ...trace.SpanStartOption) error {
	fileTag := getFileTag(1)
	ctx = startSpan(ctx, spanName, fileTag, ErrorLevel, opts...)
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
	spanName, fileTag := getCallerSpanName(1)
	ctx = startSpan(ctx, spanName, fileTag, InfoLevel, opts...)
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
	spanName, fileTag := getCallerSpanName(1)
	ctx = startSpan(ctx, spanName, fileTag, DebugLevel, opts...)
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
	spanName, fileTag := getCallerSpanName(1)
	ctx = startSpan(ctx, spanName, fileTag, InfoLevel, opts...)
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
	spanName, fileTag := getCallerSpanName(1)
	ctx = startSpan(ctx, spanName, fileTag, WarnLevel, opts...)
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
	spanName, fileTag := getCallerSpanName(1)
	ctx = startSpan(ctx, spanName, fileTag, ErrorLevel, opts...)
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
	fileTag := getFileTag(1)
	ctx = startSpan(ctx, spanName, fileTag, InfoLevel, opts...)
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
	fileTag := getFileTag(1)
	ctx = startSpan(ctx, spanName, fileTag, DebugLevel, opts...)
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
	fileTag := getFileTag(1)
	ctx = startSpan(ctx, spanName, fileTag, InfoLevel, opts...)
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
	fileTag := getFileTag(1)
	ctx = startSpan(ctx, spanName, fileTag, WarnLevel, opts...)
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
	fileTag := getFileTag(1)
	ctx = startSpan(ctx, spanName, fileTag, ErrorLevel, opts...)
	err := action(ctx)
	EndScope(ctx, err)
	return err
}

func startScope(ctx context.Context, level Level, skipCallers int, opts ...trace.SpanStartOption) context.Context {
	if level == ErrorLevel {
		opts = append(opts, trace.WithAttributes(
			attribute.Bool("error", true),
		))
	}

	spanName, fileTag := getCallerSpanName(skipCallers + 1)
	return startSpan(ctx, spanName, fileTag, level, opts...)
}

func startNamedScope(ctx context.Context, spanName string, level Level, skipCallers int, opts ...trace.SpanStartOption) context.Context {
	if level == ErrorLevel {
		opts = append(opts, trace.WithAttributes(
			attribute.Bool("error", true),
		))
	}

	fileTag := getFileTag(skipCallers + 1)
	return startSpan(ctx, spanName, fileTag, level, opts...)
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

// getCallerSpanName returns function and file name:line of the caller.
//
// Use skip=0 to get the caller of getCallerSpanName.
//
// Use skip=1 to get the caller of the caller(a getCallerSpanName() wrapper) and so on.
func getCallerSpanName(skip int) (spanName, fileTag string) {
	pc, file, line, ok := runtime.Caller(skip + 1)

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

// getFileTag returns file name:line of the caller.
//
// Use skip=0 to get the caller of getFileTag.
//
// Use skip=1 to get the caller of the caller(a getFileTag() wrapper).
func getFileTag(skip int) string {
	_, file, line, ok := runtime.Caller(skip + 1)

	// Determine source file and line number.
	if !ok {
		// Rare condition.  Probably a bug in caller.
		return "unknown"
	}

	return file + ":" + strconv.Itoa(line)
}
