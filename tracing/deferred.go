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
	name     string
	err      error
	start    time.Time
	end      time.Time
	opts     []trace.SpanStartOption
	children []*DeferredSpan
	flushed  bool
}

// NewTracer creates a DeferredTracer instance.
func NewTracer() *DeferredTracer {
	return new(DeferredTracer)
}

func (t *DeferredTracer) StartSpan(opts ...trace.SpanStartOption) *DeferredSpan {
	start := clock.Now()
	name, fileTag := getCallerSpanName(1)
	opts = append(opts, trace.WithAttributes(
		attribute.String("file", fileTag),
	))
	span := &DeferredSpan{
		name:  name,
		start: start,
		opts:  opts,
	}
	t.spans = append(t.spans, span)
	return span
}

func (t *DeferredTracer) StartNamedSpan(name string, opts ...trace.SpanStartOption) *DeferredSpan {
	start := clock.Now()
	fileTag := getFileTag(1)
	opts = append(opts, trace.WithAttributes(
		attribute.String("file", fileTag),
	))
	span := &DeferredSpan{
		name:  name,
		start: start,
		opts:  opts,
	}
	t.spans = append(t.spans, span)
	return span
}

// Flush sends all deferred events to OpenTelemetry.
// Deferred spans will be created as children to span assigned to ctx.
// Returns true if any spans are still open.
func (t *DeferredTracer) Flush(ctx context.Context) bool {
	var keepSpans []*DeferredSpan

	for _, span := range t.spans {
		remaining := t.flushSpan(ctx, span)
		if len(remaining) > 0 {
			keepSpans = append(keepSpans, remaining...)
		}
	}

	t.spans = keepSpans
	return len(t.spans) > 0
}

// flushSpan sends a span and its children to OpenTelemetry.  Any unclosed
// spans are returned in remaining return value.
func (t *DeferredTracer) flushSpan(ctx context.Context, span *DeferredSpan) (remaining []*DeferredSpan) {
	if span.end.IsZero() {
		// Preserve unclosed spans.
		remaining = append(remaining, span)
	} else if !span.flushed {
		// Create real OpenTelemetry span.
		opts := append(span.opts, trace.WithTimestamp(span.start))
		ctx2 := StartNamedScope(ctx, span.name, opts...)
		realspan := trace.SpanFromContext(ctx2)
		if span.err != nil {
			realspan.RecordError(span.err)
			realspan.SetStatus(codes.Error, span.err.Error())
		}
		realspan.End(trace.WithTimestamp(span.end))
		span.flushed = true
	}

	// Traverse children.
	childrenRemaining := false
	for _, childSpan := range span.children {
		childRemaining := t.flushSpan(ctx, childSpan)
		if len(childRemaining) > 0 {
			remaining = append(remaining, childRemaining...)
			childrenRemaining = true
		}
	}
	if !childrenRemaining {
		span.children = span.children[:0]
	}

	return
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
	opts = append(opts, trace.WithAttributes(
		attribute.String("file", fileTag),
	))
	childSpan := &DeferredSpan{
		name:  name,
		start: start,
		opts:  opts,
	}
	span.children = append(span.children, childSpan)
	return childSpan
}

func (span *DeferredSpan) StartNamedChildSpan(name string, opts ...trace.SpanStartOption) *DeferredSpan {
	start := clock.Now()
	fileTag := getFileTag(1)
	opts = append(opts, trace.WithAttributes(
		attribute.String("file", fileTag),
	))
	childSpan := &DeferredSpan{
		name:  name,
		start: start,
		opts:  opts,
	}
	span.children = append(span.children, childSpan)
	return childSpan
}
