package tracing_test

import (
	"context"
	"os"
	"testing"

	"github.com/mailgun/holster/v4/tracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
)

func TestDeferredTracer(t *testing.T) {
	ctx := context.Background()
	initTracing := func(t *testing.T) {
		os.Setenv("OTEL_TRACES_EXPORTER", "test")
		defer os.Unsetenv("OTEL_TRACES_EXPORTER")
		err := tracing.InitTracing(ctx, t.Name())
		require.NoError(t, err)
	}

	t.Run("Single span", func(t *testing.T) {
		// Given
		initTracing(t)
		prefix := t.Name() + "_"

		// When
		dtracer := tracing.NewDeferredTracer()
		span1 := dtracer.StartNamedSpan(prefix + "Span 1")
		span1.EndSpan(nil)
		remaining := dtracer.Flush(ctx)
		err := tracing.CloseTracing(ctx)
		require.NoError(t, err)

		// Then
		assert.Zero(t, remaining)
		count := tracing.GlobalTestExporter.Count()
		assert.Equal(t, 1, count)
		reader := tracing.GlobalTestExporter.NewSpanReader()
		spans := make([]trace.ReadOnlySpan, count)
		n, err := reader.Read(spans)
		require.NoError(t, err)
		assert.Equal(t, count, n)
		assert.Equal(t, prefix+"Span 1", spans[0].Name())
	})

	t.Run("Multiple spans", func(t *testing.T) {
		// Given
		initTracing(t)
		prefix := t.Name() + "_"

		// When
		dtracer := tracing.NewDeferredTracer()
		span1 := dtracer.StartNamedSpan(prefix + "Span 1")
		span1.EndSpan(nil)
		span2 := dtracer.StartNamedSpan(prefix + "Span 2")
		span2.EndSpan(nil)
		span3 := dtracer.StartNamedSpan(prefix + "Span 3")
		span3.EndSpan(nil)
		remaining := dtracer.Flush(ctx)
		err := tracing.CloseTracing(ctx)
		require.NoError(t, err)

		// Then
		assert.Zero(t, remaining)
		count := tracing.GlobalTestExporter.Count()
		assert.Equal(t, 3, count)
		reader := tracing.GlobalTestExporter.NewSpanReader()
		spans := make([]trace.ReadOnlySpan, count)
		n, err := reader.Read(spans)
		require.NoError(t, err)
		assert.Equal(t, count, n)
		assert.Equal(t, prefix+"Span 1", spans[0].Name())
		assert.Equal(t, prefix+"Span 2", spans[1].Name())
		assert.Equal(t, prefix+"Span 3", spans[2].Name())

		// Check same parent span IDs.
		assert.Equal(t, spans[0].Parent().SpanID(), spans[1].Parent().SpanID())
		assert.Equal(t, spans[0].Parent().SpanID(), spans[2].Parent().SpanID())
	})

	t.Run("Nested spans", func(t *testing.T) {
		// Given
		initTracing(t)
		prefix := t.Name() + "_"

		// When
		dtracer := tracing.NewDeferredTracer()
		span1 := dtracer.StartNamedSpan(prefix + "Span 1")
		span2 := span1.StartNamedChildSpan(prefix + "Span 2")
		span3 := span2.StartNamedChildSpan(prefix + "Span 3")
		span3.EndSpan(nil)
		span2.EndSpan(nil)
		span1.EndSpan(nil)
		remaining := dtracer.Flush(ctx)
		err := tracing.CloseTracing(ctx)
		require.NoError(t, err)

		// Then
		assert.Zero(t, remaining)
		count := tracing.GlobalTestExporter.Count()
		assert.Equal(t, 3, count)
		reader := tracing.GlobalTestExporter.NewSpanReader()
		spans := make([]trace.ReadOnlySpan, count)
		n, err := reader.Read(spans)
		require.NoError(t, err)
		assert.Equal(t, count, n)
		assert.Equal(t, prefix+"Span 1", spans[0].Name())
		assert.Equal(t, prefix+"Span 2", spans[1].Name())
		assert.Equal(t, prefix+"Span 3", spans[2].Name())

		// Check parent/child span IDs.
		assert.Equal(t, spans[1].SpanContext().SpanID(), spans[2].Parent().SpanID())
		assert.Equal(t, spans[0].SpanContext().SpanID(), spans[1].Parent().SpanID())
	})
}
