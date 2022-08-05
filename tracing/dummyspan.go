package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// DummySpan is used to create a span that functionally does nothing, but can
// be nested within non-dummy spans.
// This is used to selectively disable tracing individual spans based on
// criteria, such as log level.
type DummySpan struct {
	parentSpan trace.Span
}

// newDummySpan creates a dummy span as a child of provided parent span.
func newDummySpan(ctx context.Context) (context.Context, trace.Span) {
	parentSpan := trace.SpanFromContext(ctx)

	// If dummy spans are nested, traverse up to next non-dummy span to find
	// parent.
	if parentSpan2, ok := parentSpan.(*DummySpan); ok {
		parentSpan = parentSpan2.parentSpan
	}

	span := &DummySpan{
		parentSpan: parentSpan,
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
