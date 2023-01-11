package tracing

import (
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

// NewHTTPClient creates an HTTP client configured with OpenTelemetry
// span proagation.
func NewHTTPClient() *http.HTTPClient {
	opts := []otelhttp.Option{
		otelhttp.WithSpanOptions(trace.WithSpanKind(trace.SpanKindClient)),
		otelhttp.WithSpanNameFormatter(spanNameFormatter),
	}
	return &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport, opts...),
	}
}

func spanNameFormatter(_ string, r *http.Request) string {
	return r.Method + " " + r.URL.Path
}
