package tracing_test

import (
	"context"
	"testing"
	"time"

	"github.com/mailgun/holster/v4/errors"
	"github.com/mailgun/holster/v4/tracing"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func TestTracing(t *testing.T) {
	ctx := context.Background()
	tracer, err := tracing.InitTracing("TestTracing")
	require.NoError(t, err)
	defer func() {
		err := tracing.CloseTracing(ctx)
		require.NoError(t, err)
		time.Sleep(2 * time.Second)
	}()

	t.Run("Simple traces", func(t *testing.T) {
		ctx, span := tracer.Start(ctx, "Send traces")
		defer span.End()

		for i := 0; i < 10; i++ {
			_, span2 := tracer.Start(ctx, "Span", trace.WithAttributes(
				attribute.Int("iteration", i),
			))
			time.Sleep(10 * time.Millisecond)
			span2.End()
		}
	})

	t.Run("Log to trace", func(t *testing.T) {
		ctx, span := tracer.Start(ctx, "Log to trace")
		defer span.End()

		logrus.WithContext(ctx).
			WithError(errors.New("Test error")).
			WithField("testId", 12345).
			Error("This is a log message")
	})

	t.Run("Scope()", func(t *testing.T) {
		t.Run("Simple traces", func(t *testing.T) {
			err := tracing.Scope(ctx, "Scope", func(s *tracing.S) error {
				for i := 0; i < 10; i++ {
					err := tracing.Scope(s.Ctx, "Span", func(s *tracing.S) error {
						time.Sleep(10 * time.Millisecond)
						return nil
					})

					require.NoError(t, err)
				}

				return nil
			})

			require.NoError(t, err)
		})

		t.Run("Return error", func(t *testing.T) {
			err := tracing.Scope(ctx, "Error scope", func(s *tracing.S) error {
				time.Sleep(1 * time.Millisecond)
				return errors.New("Test error")
			})

			require.Error(t, err)
		})
	})
}
