package tracing_test

import (
	"context"
	"testing"

	"github.com/mailgun/holster/v4/errors"
	"github.com/mailgun/holster/v4/tracing"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestTracing(t *testing.T) {
	ctx := context.Background()
	logrus.SetLevel(logrus.DebugLevel)
	t.Setenv("OTEL_TRACES_SAMPLER", "always_off")

	t.Run("InitTracing()", func(t *testing.T) {
		t.Run("Happy path", func(t *testing.T) {
			err := tracing.InitTracing(ctx, "TestTracing")
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
			res, err := tracing.NewResource("Foobar service", "v1.0.0")
			require.NoError(t, err)

			err = tracing.InitTracing(ctx, "TestTracing", tracing.WithResource(res))
			require.NoError(t, err)

			err = tracing.CloseTracing(ctx)
			require.NoError(t, err)
		})
	})

	t.Run("Manual tracing", func(t *testing.T) {
		err := tracing.InitTracing(ctx, "TestTracing")
		require.NoError(t, err)
		defer func() {
			err := tracing.CloseTracing(ctx)
			require.NoError(t, err)
		}()
		tracer := tracing.Tracer()

		t.Run("Simple traces", func(t *testing.T) {
			ctx, span := tracer.Start(ctx, t.Name())
			defer span.End()
			assertValid(t, ctx)

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

	startScopeTestCases := []struct {
		Name string
		Fn   func(ctx context.Context, opts ...trace.SpanStartOption) context.Context
	}{
		{Name: "StartScope()", Fn: tracing.StartScope},
		{Name: "StartScopeDebug()", Fn: tracing.StartScopeDebug},
		{Name: "StartScopeInfo()", Fn: tracing.StartScopeInfo},
		{Name: "StartScopeWarn()", Fn: tracing.StartScopeWarn},
		{Name: "StartScopeError()", Fn: tracing.StartScopeError},
		{
			Name: "StartNamedScope()",
			Fn: func(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
				return tracing.StartNamedScope(ctx, t.Name()+"/StartNamedScope()", opts...)
			},
		},
		{
			Name: "StartNamedScopeDebug()",
			Fn: func(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
				return tracing.StartNamedScopeDebug(ctx, t.Name()+"/StartNamedScopeDebug()", opts...)
			},
		},
		{
			Name: "StartNamedScopeInfo()",
			Fn: func(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
				return tracing.StartNamedScopeInfo(ctx, t.Name()+"/StartNamedScopeInfo()", opts...)
			},
		},
		{
			Name: "StartNamedScopeWarn()",
			Fn: func(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
				return tracing.StartNamedScopeWarn(ctx, t.Name()+"/StartNamedScopeWarn()", opts...)
			},
		},
		{
			Name: "StartNamedScopeError()",
			Fn: func(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
				return tracing.StartNamedScopeError(ctx, t.Name()+"/StartNamedScopeError()", opts...)
			},
		},
	}

	for _, testCase := range startScopeTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			err := tracing.InitTracing(ctx, "TestTracing")
			require.NoError(t, err)
			defer func() {
				err := tracing.CloseTracing(ctx)
				require.NoError(t, err)
			}()

			t.Run("Simple traces", func(t *testing.T) {
				err := tracing.CallNamedScope(ctx, t.Name(), func(ctx context.Context) error {
					assertValid(t, ctx)
					for i := 0; i < 10; i++ {
						ctx = testCase.Fn(ctx)
						assertValid(t, ctx)
						require.NotNil(t, ctx)
						tracing.EndScope(ctx, nil)
					}

					return nil
				})

				require.NoError(t, err)
			})

			t.Run("Log to trace", func(t *testing.T) {
				ctx = testCase.Fn(ctx)
				defer tracing.EndScope(ctx, nil)
				logrus.WithContext(ctx).
					WithField("testId", 12345).
					Info("This is a log message")
				logrus.WithContext(ctx).
					WithError(errors.New("Test error")).
					Error("This is an error message")
			})

			t.Run("Add attributes to span", func(t *testing.T) {
				ctx = testCase.Fn(ctx)
				defer tracing.EndScope(ctx, nil)
				span := trace.SpanFromContext(ctx)
				span.SetAttributes(
					attribute.String("foobar_string", "Hello world."),
					attribute.Int("foobar_number", 12345),
				)
			})
		})
	}

	callScopeTestCases := []struct {
		Name string
		Fn   func(context.Context, tracing.ScopeAction, ...trace.SpanStartOption) error
	}{
		{Name: "CallScope()", Fn: tracing.CallScope},
		{Name: "CallScopeDebug()", Fn: tracing.CallScopeDebug},
		{Name: "CallScopeInfo()", Fn: tracing.CallScopeInfo},
		{Name: "CallScopeWarn()", Fn: tracing.CallScopeWarn},
		{Name: "CallScopeError()", Fn: tracing.CallScopeError},
		{
			Name: "CallNamedScope()",
			Fn: func(ctx context.Context, action tracing.ScopeAction, opts ...trace.SpanStartOption) error {
				return tracing.CallNamedScope(ctx, t.Name()+"/CallNamedScope()", action, opts...)
			},
		},
		{
			Name: "CallNamedScopeDebug()",
			Fn: func(ctx context.Context, action tracing.ScopeAction, opts ...trace.SpanStartOption) error {
				return tracing.CallNamedScopeDebug(ctx, t.Name()+"/CallNamedScopeDebug()", action, opts...)
			},
		},
		{
			Name: "CallNamedScopeInfo()",
			Fn: func(ctx context.Context, action tracing.ScopeAction, opts ...trace.SpanStartOption) error {
				return tracing.CallNamedScopeInfo(ctx, t.Name()+"/CallNamedScopeInfo()", action, opts...)
			},
		},
		{
			Name: "CallNamedScopeWarn()",
			Fn: func(ctx context.Context, action tracing.ScopeAction, opts ...trace.SpanStartOption) error {
				return tracing.CallNamedScopeWarn(ctx, t.Name()+"/CallNamedScopeWarn()", action, opts...)
			},
		},
		{
			Name: "CallNamedScopeError()",
			Fn: func(ctx context.Context, action tracing.ScopeAction, opts ...trace.SpanStartOption) error {
				return tracing.CallNamedScopeError(ctx, t.Name()+"/CallNamedScopeError()", action, opts...)
			},
		},
	}

	for _, testCase := range callScopeTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			err := tracing.InitTracing(ctx, "TestTracing")
			require.NoError(t, err)
			defer func() {
				err := tracing.CloseTracing(ctx)
				require.NoError(t, err)
			}()

			t.Run("Simple traces", func(t *testing.T) {
				err := tracing.CallNamedScope(ctx, t.Name(), func(ctx context.Context) error {
					assertValid(t, ctx)
					for i := 0; i < 10; i++ {
						err := testCase.Fn(ctx, func(_ context.Context) error {
							assertValid(t, ctx)
							return nil
						})

						require.NoError(t, err)
					}

					return nil
				})

				require.NoError(t, err)
			})

			t.Run("Log to trace", func(t *testing.T) {
				err := testCase.Fn(ctx, func(ctx context.Context) error {
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
				err := testCase.Fn(ctx, func(_ context.Context) error {
					return errors.New("Test error")
				})

				require.Error(t, err)
			})

			t.Run("Add attributes to span", func(t *testing.T) {
				err := testCase.Fn(ctx, func(ctx context.Context) error {
					span := trace.SpanFromContext(ctx)
					span.SetAttributes(
						attribute.String("foobar_string", "Hello world."),
						attribute.Int("foobar_number", 12345),
					)
					return nil
				})

				require.NoError(t, err)
			})
		})
	}

	branchScopeTestCases := []struct {
		Name string
		Fn   func(ctx context.Context, opts ...trace.SpanStartOption) context.Context
	}{
		{Name: "BranchScope()", Fn: tracing.BranchScope},
		{Name: "BranchScopeDebug()", Fn: tracing.BranchScopeDebug},
		{Name: "BranchScopeInfo()", Fn: tracing.BranchScopeInfo},
		{Name: "BranchScopeWarn()", Fn: tracing.BranchScopeWarn},
		{Name: "BranchScopeError()", Fn: tracing.BranchScopeError},
		{
			Name: "BranchNamedScope()",
			Fn: func(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
				return tracing.BranchNamedScope(ctx, t.Name()+"/BranchNamedScope()", opts...)
			},
		},
		{
			Name: "BranchNamedScopeDebug()",
			Fn: func(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
				return tracing.BranchNamedScopeDebug(ctx, t.Name()+"/BranchNamedScopeDebug()", opts...)
			},
		},
		{
			Name: "BranchNamedScopeInfo()",
			Fn: func(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
				return tracing.BranchNamedScopeInfo(ctx, t.Name()+"/BranchNamedScopeInfo()", opts...)
			},
		},
		{
			Name: "BranchNamedScopeWarn()",
			Fn: func(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
				return tracing.BranchNamedScopeWarn(ctx, t.Name()+"/BranchNamedScopeWarn()", opts...)
			},
		},
		{
			Name: "BranchNamedScopeError()",
			Fn: func(ctx context.Context, opts ...trace.SpanStartOption) context.Context {
				return tracing.BranchNamedScopeError(ctx, t.Name()+"/BranchNamedScopeError()", opts...)
			},
		},
	}

	for _, testCase := range branchScopeTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			err := tracing.InitTracing(ctx, "TestTracing")
			require.NoError(t, err)
			defer func() {
				err := tracing.CloseTracing(ctx)
				require.NoError(t, err)
			}()

			t.Run("Happy path", func(t *testing.T) {
				assertValid(t, ctx)
				// Create parent span.
				err := tracing.CallNamedScope(ctx, t.Name(), func(ctx context.Context) error {
					// Eval test case.
					ctx = testCase.Fn(ctx)
					assertValid(t, ctx)
					tracing.EndScope(ctx, nil)
					return nil
				})

				require.NoError(t, err)
			})

			t.Run("No op when no parent span", func(t *testing.T) {
				ctx := context.Background()
				assertNotValid(t, ctx)
				ctx2 := testCase.Fn(ctx)
				defer tracing.EndScope(ctx2, nil)
				assertNotValid(t, ctx2)
			})
		})
	}

	callScopeBranchTestCases := []struct {
		Name string
		Fn   func(context.Context, tracing.ScopeAction, ...trace.SpanStartOption) error
	}{
		{Name: "CallScopeBranch()", Fn: tracing.CallScopeBranch},
		{Name: "CallScopeBranchDebug()", Fn: tracing.CallScopeBranchDebug},
		{Name: "CallScopeBranchInfo()", Fn: tracing.CallScopeBranchInfo},
		{Name: "CallScopeBranchWarn()", Fn: tracing.CallScopeBranchWarn},
		{Name: "CallScopeBranchError()", Fn: tracing.CallScopeBranchError},
		{
			Name: "CallNamedScopeBranch()",
			Fn: func(ctx context.Context, action tracing.ScopeAction, opts ...trace.SpanStartOption) error {
				return tracing.CallNamedScopeBranch(ctx, t.Name()+"/CallNamedScopeBranch()", action, opts...)
			},
		},
		{
			Name: "CallNamedScopeBranchDebug()",
			Fn: func(ctx context.Context, action tracing.ScopeAction, opts ...trace.SpanStartOption) error {
				return tracing.CallNamedScopeBranchDebug(ctx, t.Name()+"/CallNamedScopeBranchDebug()", action, opts...)
			},
		},
		{
			Name: "CallNamedScopeBranchInfo()",
			Fn: func(ctx context.Context, action tracing.ScopeAction, opts ...trace.SpanStartOption) error {
				return tracing.CallNamedScopeBranchInfo(ctx, t.Name()+"/CallNamedScopeBranchInfo()", action, opts...)
			},
		},
		{
			Name: "CallNamedScopeBranchWarn()",
			Fn: func(ctx context.Context, action tracing.ScopeAction, opts ...trace.SpanStartOption) error {
				return tracing.CallNamedScopeBranchWarn(ctx, t.Name()+"/CallNamedScopeBranchWarn()", action, opts...)
			},
		},
		{
			Name: "CallNamedScopeBranchError()",
			Fn: func(ctx context.Context, action tracing.ScopeAction, opts ...trace.SpanStartOption) error {
				return tracing.CallNamedScopeBranchError(ctx, t.Name()+"/CallNamedScopeBranchError()", action, opts...)
			},
		},
	}

	for _, testCase := range callScopeBranchTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			err := tracing.InitTracing(ctx, "TestTracing")
			require.NoError(t, err)
			defer func() {
				err := tracing.CloseTracing(ctx)
				require.NoError(t, err)
			}()

			t.Run("Happy path", func(t *testing.T) {
				// Create parent span.
				err := tracing.CallNamedScope(ctx, t.Name(), func(ctx2 context.Context) error {
					assertValid(t, ctx2)
					// Eval test case.
					err := testCase.Fn(ctx2, func(ctx3 context.Context) error {
						assertValid(t, ctx3)
						return nil
					})

					require.NoError(t, err)
					return nil
				})

				require.NoError(t, err)
			})

			t.Run("No op when no parent span", func(t *testing.T) {
				ctx := context.Background()
				assertNotValid(t, ctx)
				err := testCase.Fn(ctx, func(ctx2 context.Context) error {
					assertNotValid(t, ctx2)
					return nil
				})

				require.NoError(t, err)
			})
		})
	}
}

func assertValid(t *testing.T, ctx context.Context) {
	assert.True(t, trace.SpanContextFromContext(ctx).IsValid())
}

func assertNotValid(t *testing.T, ctx context.Context) {
	assert.False(t, trace.SpanContextFromContext(ctx).IsValid())
}
