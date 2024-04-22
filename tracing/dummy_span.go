package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
)

// DummySpan is used to create a stub span as a placeholder for spans not
// intended for export.
// This is used to selectively disable tracing individual spans based on
// criteria, such as log level.
// Any child spans created from a DummySpan will be linked to the dummy's
// next non-dummy ancestor.
type DummySpan struct {
	embedded.Span

	// Next non-dummy parent span.
	parentSpan trace.Span
}

// newDummySpan creates a dummy span as a child of provided parent span.
func newDummySpan(ctx context.Context) (context.Context, trace.Span) {
	span := &DummySpan{
		parentSpan: trace.SpanFromContext(ctx),
	}
	ctx = trace.ContextWithSpan(ctx, span)

	return ctx, span
}

func (s *DummySpan) End(options ...trace.SpanEndOption) {
	// no-op
}

func (s *DummySpan) AddEvent(name string, options ...trace.EventOption) {
	// no-op
}

func (s *DummySpan) IsRecording() bool {
	return s.parentSpan.IsRecording()
}

func (s *DummySpan) RecordError(err error, options ...trace.EventOption) {
	// no-op
}

func (s *DummySpan) SpanContext() trace.SpanContext {
	return s.parentSpan.SpanContext()
}

func (s *DummySpan) SetStatus(code codes.Code, description string) {
	// no-op
}

func (s *DummySpan) SetName(name string) {
	// no-op
}

func (s *DummySpan) SetAttributes(kv ...attribute.KeyValue) {
	// no-op
}

func (s *DummySpan) TracerProvider() trace.TracerProvider {
	return s.parentSpan.TracerProvider()
}

func (s *DummySpan) AddLink(trace.Link) {
	// no-op
}
