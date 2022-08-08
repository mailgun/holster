package tracing

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Filter processor implements sdktrace.SpanProcessor to filter spans.
// Filter by span log level attribute.
// Set attribute `log.level=<n>`, where `n` is RFC5424 log level number 0-7.
type filterProcessor struct {
	next  sdktrace.SpanProcessor
	level int64
}

const LogLevelKey = "log.level"

// NewFilterProcessor creates a SpanProcessor that filters by span log level
// attribute `log.level`.
func NewFilterProcessor(level int64, next sdktrace.SpanProcessor) *filterProcessor {
	return &filterProcessor{
		next:  next,
		level: int64(level),
	}
}

func (p *filterProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	p.next.OnStart(parent, s)
}

func (p *filterProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	// Parse log level from span attributes.
	for _, attr := range s.Attributes() {
		if string(attr.Key) == LogLevelKey {
			n := attr.Value.AsInt64()
			if n > p.level {
				// Drop span.
				return
			}
			break
		}
	}

	// Filter span.
	p.next.OnEnd(s)
}

func (p *filterProcessor) Shutdown(ctx context.Context) error {
	return p.next.Shutdown(ctx)
}

func (p *filterProcessor) ForceFlush(ctx context.Context) error {
	return p.next.ForceFlush(ctx)
}
