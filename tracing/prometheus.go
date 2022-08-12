package tracing

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// MetricSpanProcessor implements SpanProcessor as a middleware to track
// via Prometheus metrics before export.
type MetricSpanProcessor struct {
	next sdktrace.SpanProcessor
}

var (
	traceCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "holster_tracing_counter",
		Help: "Count of traces generated by holster `tracing` package.",
	}, []string{"error"})
	spanCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "holster_tracing_spans",
		Help: "Count of trace spans generated by holster `tracing` package.",
	}, []string{"error"})

	// Metrics contains Prometheus metrics available for use.
	Metrics = []prometheus.Collector{
		traceCounter,
		spanCounter,
	}
)

func NewMetricSpanProcessor(next sdktrace.SpanProcessor) *MetricSpanProcessor {
	return &MetricSpanProcessor{
		next: next,
	}
}

func (p *MetricSpanProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	p.next.OnStart(parent, s)
}

func (p *MetricSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	var errorVec string
	if s.Status().Code == codes.Error {
		errorVec = "true"
	}

	if !s.Parent().HasTraceID() {
		traceCounter.WithLabelValues(errorVec).Add(1)
	}

	spanCounter.WithLabelValues(errorVec).Add(1)

	p.next.OnEnd(s)
}

func (p *MetricSpanProcessor) Shutdown(ctx context.Context) error {
	return p.next.Shutdown(ctx)
}

func (p *MetricSpanProcessor) ForceFlush(ctx context.Context) error {
	return p.next.ForceFlush(ctx)
}
