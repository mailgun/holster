package tracing_test

import (
	"context"

	"github.com/stretchr/testify/mock"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// MockSpanProcessor mocks interface SpanProcessor.
type MockSpanProcessor struct {
	mock.Mock
}

func (m *MockSpanProcessor) OnStart(ctx context.Context, s sdktrace.ReadWriteSpan) {
	m.Called(ctx, s)
}

func (m *MockSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	m.Called(s)
}

func (m *MockSpanProcessor) ForceFlush(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSpanProcessor) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
