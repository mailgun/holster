package tracing_test

import (
	"context"
	"testing"

	"github.com/mailgun/holster/v4/errors"
	"github.com/mailgun/holster/v4/tracing"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

func TestTracing(t *testing.T) {
	ctx := context.Background()
	logrus.SetLevel(logrus.DebugLevel)

	t.Run("InitTracing()", func(t *testing.T) {
		t.Run("Happy path", func(t *testing.T) {
			ctx, tracer, err := tracing.InitTracing(ctx, "TestTracing")
			require.NotNil(t, ctx)
			require.NotNil(t, tracer)
			require.NoError(t, err)

			err = tracing.CloseTracing(ctx)
			require.NoError(t, err)
		})

		t.Run("Set service name", func(t *testing.T) {
			// Sets service name resource.
			// This overrides environment variable `OTEL_SERVICE_NAME`.
			// If neither provided, default service name is
			// "unknown_service:<executable-filename>".
			// See: https://opentelemetry.io/docs/instrumentation/go/getting-started/#creating-a-resource
			res, err := resource.Merge(
				resource.Default(),
				resource.NewWithAttributes(
					semconv.SchemaURL,
					semconv.ServiceNameKey.String("Foobar service"),
					semconv.ServiceVersionKey.String("v1.0.0"),
				),
			)
			require.NoError(t, err)

			ctx, tracer, err := tracing.InitTracing(ctx, "TestTracing", sdktrace.WithResource(res))
			require.NotNil(t, ctx)
			require.NotNil(t, tracer)
			require.NoError(t, err)

			err = tracing.CloseTracing(ctx)
			require.NoError(t, err)
		})
	})

	t.Run("Manual tracing", func(t *testing.T) {
		ctx, tracer, err := tracing.InitTracing(ctx, "TestTracing")
		require.NoError(t, err)
		defer func() {
			err := tracing.CloseTracing(ctx)
			require.NoError(t, err)
		}()

		t.Run("Simple traces", func(t *testing.T) {
			ctx, span := tracer.Start(ctx, t.Name())
			defer span.End()

			for i := 0; i < 10; i++ {
				_, span2 := tracer.Start(ctx, "", trace.WithAttributes(
					attribute.Int("iteration", i),
				))
				span2.End()
			}
		})

		t.Run("Log to trace", func(t *testing.T) {
			ctx, span := tracer.Start(ctx, t.Name())
			defer span.End()

			logrus.WithContext(ctx).
				WithField("testId", 12345).
				Info("This is a log message")
			logrus.WithContext(ctx).
				WithError(errors.New("Test error")).
				Error("This is an error message")
		})

		t.Run("Return error", func(t *testing.T) {
			_, span := tracer.Start(ctx, t.Name())
			defer span.End()

			err := errors.New("Test error")
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		})

		t.Run("Add attributes to span", func(t *testing.T) {
			_, span := tracer.Start(ctx, t.Name())
			defer span.End()

			span.SetAttributes(
				attribute.String("foobar_string", "Hello world."),
				attribute.Int("foobar_number", 12345),
			)
		})
	})

	t.Run("Scope()", func(t *testing.T) {
		ctx, _, err := tracing.InitTracing(ctx, "TestTracing")
		require.NoError(t, err)
		defer func() {
			err := tracing.CloseTracing(ctx)
			require.NoError(t, err)
		}()

		t.Run("Simple traces", func(t *testing.T) {
			err := tracing.NamedScope(ctx, t.Name(), func(ctx context.Context) error {
				for i := 0; i < 10; i++ {
					err := tracing.Scope(ctx, func(_ context.Context) error {
						return nil
					})

					require.NoError(t, err)
				}

				return nil
			})

			require.NoError(t, err)
		})

		t.Run("Log to trace", func(t *testing.T) {
			err := tracing.NamedScope(ctx, t.Name(), func(ctx context.Context) error {
				logrus.WithContext(ctx).
					WithField("testId", 12345).
					Info("This is a log message")
				logrus.WithContext(ctx).
					WithError(errors.New("Test error")).
					Error("This is an error message")
				return nil
			})

			require.NoError(t, err)
		})

		t.Run("Return error", func(t *testing.T) {
			err := tracing.NamedScope(ctx, t.Name(), func(_ context.Context) error {
				return errors.New("Test error")
			})

			require.Error(t, err)
		})

		t.Run("Add attributes to span", func(t *testing.T) {
			err := tracing.NamedScope(ctx, t.Name(), func(ctx context.Context) error {
				span := trace.SpanFromContext(ctx)
				span.SetAttributes(
					attribute.String("foobar_string", "Hello world."),
					attribute.Int("foobar_number", 12345),
				)
				return nil
			})

			require.NoError(t, err)
		})

		t.Run("Custom library name", func(t *testing.T) {
			const libraryName = "Foobar library"
			ctx, _, err := tracing.NewTracer(ctx, libraryName)
			require.NoError(t, err)

			err = tracing.NamedScope(ctx, t.Name(), func(ctx context.Context) error {
				return nil
			})

			require.NoError(t, err)
		})
	})
}
