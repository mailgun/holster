package tracing_test

import (
	"context"
	"testing"

	"github.com/mailgun/holster/v4/tracing"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestDebugTracing(t *testing.T) {
	ctx := context.Background()
	res, err := tracing.NewResource("foobar", "1.0")
	require.NoError(t, err)
	t.Setenv("OTEL_EXPORTERS", "none")

	t.Run("No levels", func(t *testing.T) {
		// Mock OTel exporter.
		mockProcessor := new(MockSpanProcessor)
		mockProcessor.On("OnStart", mock.Anything, mock.Anything).Once()
		mockProcessor.On("OnEnd", mock.Anything, mock.Anything).Once().
			Run(func(args mock.Arguments) {
				s := args.Get(0).(sdktrace.ReadOnlySpan)
				assertHasNoLogLevel(t, int64(logrus.DebugLevel), s)
			})
		mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

		logLevel := int64(logrus.InfoLevel)
		processor := tracing.NewFilterProcessor(logLevel, mockProcessor)
		_, _, err := tracing.InitTracing(ctx, "foobar",
			sdktrace.WithResource(res),
			sdktrace.WithSpanProcessor(processor),
		)
		require.NoError(t, err)

		// Create a span.
		ctx2 := tracing.StartScope(ctx)
		tracing.EndScope(ctx2, nil)

		err = tracing.CloseTracing(context.Background())
		require.NoError(t, err)

		// Verify.
		mockProcessor.AssertExpectations(t)
	})

	t.Run("Filter", func(t *testing.T) {
		testCases := []struct {
			name           string
			logLevel       logrus.Level
			spanLogLevel   logrus.Level
			expectedFilter bool
		}{
			{
				name:           "Error level is filtered",
				logLevel:       logrus.InfoLevel,
				spanLogLevel:   logrus.ErrorLevel,
				expectedFilter: true,
			},
			{
				name:           "Info level is filtered",
				logLevel:       logrus.InfoLevel,
				spanLogLevel:   logrus.InfoLevel,
				expectedFilter: true,
			},
			{
				name:           "Debug level is dropped",
				logLevel:       logrus.InfoLevel,
				spanLogLevel:   logrus.DebugLevel,
				expectedFilter: false,
			},
			{
				name:           "Debug level is filtered",
				logLevel:       logrus.DebugLevel,
				spanLogLevel:   logrus.DebugLevel,
				expectedFilter: true,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				// Mock OTel exporter.
				mockProcessor := new(MockSpanProcessor)
				mockProcessor.On("OnStart", mock.Anything, mock.Anything).Once()
				if testCase.expectedFilter {
					mockProcessor.On("OnEnd", mock.Anything, mock.Anything).Once()
				}
				mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

				processor := tracing.NewFilterProcessor(int64(testCase.logLevel), mockProcessor)
				_, _, err := tracing.InitTracing(ctx, "foobar",
					sdktrace.WithResource(res),
					sdktrace.WithSpanProcessor(processor),
				)
				require.NoError(t, err)

				// Create a span.
				ctx2 := tracing.StartScope(ctx)
				span2 := trace.SpanFromContext(ctx2)
				span2.SetAttributes(attribute.Int64("log.level", int64(testCase.spanLogLevel)))
				tracing.EndScope(ctx2, nil)

				err = tracing.CloseTracing(context.Background())
				require.NoError(t, err)

				// Verify.
				mockProcessor.AssertExpectations(t)
			})
		}
	})

	t.Run("StartScopeX()", func(t *testing.T) {
		t.Run("StartScopeDebug()", func(t *testing.T) {
			// Mock OTel exporter.
			mockProcessor := new(MockSpanProcessor)
			mockProcessor.On("OnStart", mock.Anything, mock.Anything).Once()
			mockProcessor.On("OnEnd", mock.Anything, mock.Anything).Once().
				Run(func(args mock.Arguments) {
					s := args.Get(0).(sdktrace.ReadOnlySpan)
					assertHasLogLevel(t, int64(logrus.DebugLevel), s)
				})
			mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

			processor := tracing.NewFilterProcessor(int64(logrus.DebugLevel), mockProcessor)
			_, _, err := tracing.InitTracing(ctx, "foobar",
				sdktrace.WithResource(res),
				sdktrace.WithSpanProcessor(processor),
			)
			require.NoError(t, err)

			// Create a debug span.
			ctx2 := tracing.StartScopeDebug(ctx)
			tracing.EndScope(ctx2, nil)

			err = tracing.CloseTracing(context.Background())
			require.NoError(t, err)

			// Verify.
			mockProcessor.AssertExpectations(t)
		})

		t.Run("StartScopeInfo()", func(t *testing.T) {
			// Mock OTel exporter.
			mockProcessor := new(MockSpanProcessor)
			mockProcessor.On("OnStart", mock.Anything, mock.Anything).Once()
			mockProcessor.On("OnEnd", mock.Anything, mock.Anything).Once().
				Run(func(args mock.Arguments) {
					s := args.Get(0).(sdktrace.ReadOnlySpan)
					assertHasLogLevel(t, int64(logrus.InfoLevel), s)
				})
			mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

			processor := tracing.NewFilterProcessor(int64(logrus.DebugLevel), mockProcessor)
			_, _, err := tracing.InitTracing(ctx, "foobar",
				sdktrace.WithResource(res),
				sdktrace.WithSpanProcessor(processor),
			)
			require.NoError(t, err)

			// Create an info span.
			ctx2 := tracing.StartScopeInfo(ctx)
			tracing.EndScope(ctx2, nil)

			err = tracing.CloseTracing(context.Background())
			require.NoError(t, err)

			// Verify.
			mockProcessor.AssertExpectations(t)
		})

		t.Run("StartScopeWarn()", func(t *testing.T) {
			// Mock OTel exporter.
			mockProcessor := new(MockSpanProcessor)
			mockProcessor.On("OnStart", mock.Anything, mock.Anything).Once()
			mockProcessor.On("OnEnd", mock.Anything, mock.Anything).Once().
				Run(func(args mock.Arguments) {
					s := args.Get(0).(sdktrace.ReadOnlySpan)
					assertHasLogLevel(t, int64(logrus.WarnLevel), s)
				})
			mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

			processor := tracing.NewFilterProcessor(int64(logrus.DebugLevel), mockProcessor)
			_, _, err := tracing.InitTracing(ctx, "foobar",
				sdktrace.WithResource(res),
				sdktrace.WithSpanProcessor(processor),
			)
			require.NoError(t, err)

			// Create an info span.
			ctx2 := tracing.StartScopeWarn(ctx)
			tracing.EndScope(ctx2, nil)

			err = tracing.CloseTracing(context.Background())
			require.NoError(t, err)

			// Verify.
			mockProcessor.AssertExpectations(t)
		})

		t.Run("StartScopeError()", func(t *testing.T) {
			// Mock OTel exporter.
			mockProcessor := new(MockSpanProcessor)
			mockProcessor.On("OnStart", mock.Anything, mock.Anything).Once()
			mockProcessor.On("OnEnd", mock.Anything, mock.Anything).Once().
				Run(func(args mock.Arguments) {
					s := args.Get(0).(sdktrace.ReadOnlySpan)
					assertHasLogLevel(t, int64(logrus.ErrorLevel), s)
				})
			mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

			processor := tracing.NewFilterProcessor(int64(logrus.DebugLevel), mockProcessor)
			_, _, err := tracing.InitTracing(ctx, "foobar",
				sdktrace.WithResource(res),
				sdktrace.WithSpanProcessor(processor),
			)
			require.NoError(t, err)

			// Create an info span.
			ctx2 := tracing.StartScopeError(ctx)
			tracing.EndScope(ctx2, nil)

			err = tracing.CloseTracing(context.Background())
			require.NoError(t, err)

			// Verify.
			mockProcessor.AssertExpectations(t)
		})
	})

	t.Run("StartNamedScopeX()", func(t *testing.T) {
		t.Run("StartNamedScopeDebug()", func(t *testing.T) {
			// Mock OTel exporter.
			mockProcessor := new(MockSpanProcessor)
			mockProcessor.On("OnStart", mock.Anything, mock.Anything).Once()
			mockProcessor.On("OnEnd", mock.Anything, mock.Anything).Once().
				Run(func(args mock.Arguments) {
					s := args.Get(0).(sdktrace.ReadOnlySpan)
					assertHasLogLevel(t, int64(logrus.DebugLevel), s)
				})
			mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

			processor := tracing.NewFilterProcessor(int64(logrus.DebugLevel), mockProcessor)
			_, _, err := tracing.InitTracing(ctx, "foobar",
				sdktrace.WithResource(res),
				sdktrace.WithSpanProcessor(processor),
			)
			require.NoError(t, err)

			// Create a debug span.
			ctx2 := tracing.StartNamedScopeDebug(ctx, "Foobar")
			tracing.EndScope(ctx2, nil)

			err = tracing.CloseTracing(context.Background())
			require.NoError(t, err)

			// Verify.
			mockProcessor.AssertExpectations(t)
		})

		t.Run("StartNamedScopeInfo()", func(t *testing.T) {
			// Mock OTel exporter.
			mockProcessor := new(MockSpanProcessor)
			mockProcessor.On("OnStart", mock.Anything, mock.Anything).Once()
			mockProcessor.On("OnEnd", mock.Anything, mock.Anything).Once().
				Run(func(args mock.Arguments) {
					s := args.Get(0).(sdktrace.ReadOnlySpan)
					assertHasLogLevel(t, int64(logrus.InfoLevel), s)
				})
			mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

			processor := tracing.NewFilterProcessor(int64(logrus.DebugLevel), mockProcessor)
			_, _, err := tracing.InitTracing(ctx, "foobar",
				sdktrace.WithResource(res),
				sdktrace.WithSpanProcessor(processor),
			)
			require.NoError(t, err)

			// Create an info span.
			ctx2 := tracing.StartNamedScopeInfo(ctx, "Foobar")
			tracing.EndScope(ctx2, nil)

			err = tracing.CloseTracing(context.Background())
			require.NoError(t, err)

			// Verify.
			mockProcessor.AssertExpectations(t)
		})

		t.Run("StartNamedScopeWarn()", func(t *testing.T) {
			// Mock OTel exporter.
			mockProcessor := new(MockSpanProcessor)
			mockProcessor.On("OnStart", mock.Anything, mock.Anything).Once()
			mockProcessor.On("OnEnd", mock.Anything, mock.Anything).Once().
				Run(func(args mock.Arguments) {
					s := args.Get(0).(sdktrace.ReadOnlySpan)
					assertHasLogLevel(t, int64(logrus.WarnLevel), s)
				})
			mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

			processor := tracing.NewFilterProcessor(int64(logrus.DebugLevel), mockProcessor)
			_, _, err := tracing.InitTracing(ctx, "foobar",
				sdktrace.WithResource(res),
				sdktrace.WithSpanProcessor(processor),
			)
			require.NoError(t, err)

			// Create an info span.
			ctx2 := tracing.StartNamedScopeWarn(ctx, "Foobar")
			tracing.EndScope(ctx2, nil)

			err = tracing.CloseTracing(context.Background())
			require.NoError(t, err)

			// Verify.
			mockProcessor.AssertExpectations(t)
		})

		t.Run("StartNamedScopeError()", func(t *testing.T) {
			// Mock OTel exporter.
			mockProcessor := new(MockSpanProcessor)
			mockProcessor.On("OnStart", mock.Anything, mock.Anything).Once()
			mockProcessor.On("OnEnd", mock.Anything, mock.Anything).Once().
				Run(func(args mock.Arguments) {
					s := args.Get(0).(sdktrace.ReadOnlySpan)
					assertHasLogLevel(t, int64(logrus.ErrorLevel), s)
				})
			mockProcessor.On("Shutdown", mock.Anything).Once().Return(nil)

			processor := tracing.NewFilterProcessor(int64(logrus.DebugLevel), mockProcessor)
			_, _, err := tracing.InitTracing(ctx, "foobar",
				sdktrace.WithResource(res),
				sdktrace.WithSpanProcessor(processor),
			)
			require.NoError(t, err)

			// Create an info span.
			ctx2 := tracing.StartNamedScopeError(ctx, "Foobar")
			tracing.EndScope(ctx2, nil)

			err = tracing.CloseTracing(context.Background())
			require.NoError(t, err)

			// Verify.
			mockProcessor.AssertExpectations(t)
		})
	})
}

func assertHasLogLevel(t *testing.T, expectedLogLevel int64, s sdktrace.ReadOnlySpan) {
	for _, attr := range s.Attributes() {
		if string(attr.Key) == tracing.LogLevelKey {
			n := attr.Value.AsInt64()
			assert.Equal(t, expectedLogLevel, n, "Span log level mismatch")
			return
		}
	}

	t.Error("Error: Span log level not defined, but was expected to be")
}

func assertHasNoLogLevel(t *testing.T, expectedLogLevel int64, s sdktrace.ReadOnlySpan) {
	for _, attr := range s.Attributes() {
		if string(attr.Key) == tracing.LogLevelKey {
			t.Error("Error: Span log level defined, but was expected not to be")
			return
		}
	}
}
