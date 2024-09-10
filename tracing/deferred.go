package tracing

import (
	"context"
	"time"

	"github.com/mailgun/holster/v4/clock"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// DeferredTracer collects deferred OpenTelemetry spans. Call StartSpan/EndSpan
// to build spans, then Flush to send queued events to OpenTelemetry. This
// generates trace spans at runtime but deferred until convenient to process.
// Useful for high concurrent applications where lock contention from frequent
// tracing operations would affect performance.
//
// Note: Not thread-safe.  Use in synchronous code paths only.
type DeferredTracer struct {
	spans []*DeferredSpan
}

type DeferredSpan struct {
	name        string
	err         error
	end         time.Time
	opts        []trace.SpanStartOption
	children    []*DeferredSpan
	spanContext trace.SpanContext
	flushed     bool
}

// NewDeferredTracer creates a DeferredTracer instance.
func NewDeferredTracer() *DeferredTracer {
	return new(DeferredTracer)
}

func (t *DeferredTracer) StartSpan(opts ...trace.SpanStartOption) *DeferredSpan {
	start := clock.Now()
	name, fileTag := getCallerSpanName(1)
	opts = append(opts,
		trace.WithTimestamp(start),
		trace.WithAttributes(
			attribute.String("file", fileTag),
		),
	)
	span := &DeferredSpan{
		name: name,
		opts: opts,
	}
	t.spans = append(t.spans, span)
	return span
}

func (t *DeferredTracer) StartNamedSpan(name string, opts ...trace.SpanStartOption) *DeferredSpan {
	start := clock.Now()
	fileTag := getFileTag(1)
	opts = append(opts,
		trace.WithTimestamp(start),
		trace.WithAttributes(
			attribute.String("file", fileTag),
		),
	)
	span := &DeferredSpan{
		name: name,
		opts: opts,
	}
	t.spans = append(t.spans, span)
	return span
}

// Flush sends all deferred events to OpenTelemetry.
// Deferred spans will be created as children to span assigned to ctx.
// Returns true if any spans still remain open.
func (t *DeferredTracer) Flush(ctx context.Context) (remaining bool) {
	var keepSpans []*DeferredSpan

	for _, span := range t.spans {
		if t.flushSpan(ctx, span) {
			keepSpans = append(keepSpans, span)
		}
	}

	t.spans = keepSpans
	return len(t.spans) > 0
}

// flushSpan sends a span and its children to OpenTelemetry.
// If parent span is closed, traverse child spans.
// Idempotent: flushes spans only once.  May call repeatedly until all spans
// are flushed.
// Returns true if any spans still remain open.
func (t *DeferredTracer) flushSpan(ctx context.Context, span *DeferredSpan) (remaining bool) {
	if span.end.IsZero() {
		// Return if not closed.  Do not traverse children.
		return true
	}
	if !span.flushed {
		// Create real OpenTelemetry span.
		ctx2 := StartNamedScope(ctx, span.name, span.opts...)
		realspan := trace.SpanFromContext(ctx2)
		span.spanContext = realspan.SpanContext()
		if span.err != nil {
			realspan.RecordError(span.err)
			realspan.SetStatus(codes.Error, span.err.Error())
		}
		realspan.End(trace.WithTimestamp(span.end))
		span.flushed = true
	}

	// Traverse children.
	ctx2 := trace.ContextWithSpanContext(ctx, span.spanContext)
	var keepChildren []*DeferredSpan
	for _, childSpan := range span.children {
		if t.flushSpan(ctx2, childSpan) {
			keepChildren = append(keepChildren, childSpan)
		}
	}
	span.children = keepChildren

	return len(keepChildren) > 0
}

func (span *DeferredSpan) EndSpan(err error, opts ...trace.SpanStartOption) {
	if span.end.IsZero() {
		span.err = err
		span.end = clock.Now()
		span.opts = append(span.opts, opts...)
	}
}

func (span *DeferredSpan) StartChildSpan(opts ...trace.SpanStartOption) *DeferredSpan {
	start := clock.Now()
	name, fileTag := getCallerSpanName(1)
	opts = append(opts,
		trace.WithTimestamp(start),
		trace.WithAttributes(
			attribute.String("file", fileTag),
		),
	)
	childSpan := &DeferredSpan{
		name: name,
		opts: opts,
	}
	span.children = append(span.children, childSpan)
	return childSpan
}

func (span *DeferredSpan) StartNamedChildSpan(name string, opts ...trace.SpanStartOption) *DeferredSpan {
	start := clock.Now()
	fileTag := getFileTag(1)
	opts = append(opts,
		trace.WithTimestamp(start),
		trace.WithAttributes(
			attribute.String("file", fileTag),
		),
	)
	childSpan := &DeferredSpan{
		name: name,
		opts: opts,
	}
	span.children = append(span.children, childSpan)
	return childSpan
}
