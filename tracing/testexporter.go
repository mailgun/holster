package tracing

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
)

// TestExporter used for testing.
type TestExporter struct {
	Spans []trace.ReadOnlySpan
	mutex sync.Mutex
}

func (e *TestExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	logrus.WithFields(logrus.Fields{
		"count": len(spans),
	}).Info("TestExporter.ExportSpans()")

	for _, span := range spans {
		sc := span.SpanContext()
		parentsc := span.Parent()
		logrus.WithFields(logrus.Fields{
			"name":         span.Name(),
			"traceID":      fmt.Sprintf("%x", sc.TraceID()),
			"spanID":       fmt.Sprintf("%x", sc.SpanID()),
			"parentSpanID": fmt.Sprintf("%x", parentsc.SpanID()),
			"attributes":   attrToMap(span.Attributes()),
		}).Info("Span")
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.Spans = append(e.Spans, spans...)
	return nil
}

func (e *TestExporter) Shutdown(ctx context.Context) error {
	logrus.Info("TestExporter.Shutdown()")
	return nil
}

type SpanReader struct {
	exporter *TestExporter
	index    int
}

func (e *TestExporter) Count() int {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return len(e.Spans)
}

func (e *TestExporter) NewSpanReader() *SpanReader {
	return &SpanReader{
		exporter: e,
	}
}

func (e *TestExporter) Reset() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.Spans = make([]trace.ReadOnlySpan, 0)
}

func (r *SpanReader) Read(p []trace.ReadOnlySpan) (n int, err error) {
	r.exporter.mutex.Lock()
	defer r.exporter.mutex.Unlock()

	if r.index >= len(r.exporter.Spans) {
		return 0, io.EOF
	}
	pindex := 0
	for {
		if r.index >= len(r.exporter.Spans) || pindex >= len(p) {
			return n, nil
		}
		p[pindex] = r.exporter.Spans[r.index]
		pindex++
		r.index++
		n++
	}
}

func attrToMap(attrs []attribute.KeyValue) map[string]any {
	m := make(map[string]any)
	for _, attr := range attrs {
		m[string(attr.Key)] = attr.Value.AsInterface()
	}
	return m
}
