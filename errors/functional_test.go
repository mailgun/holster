package errors_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ahmetb/go-linq"
	"github.com/mailgun/holster/v5/callstack"
	"github.com/mailgun/holster/v5/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestError struct {
	Msg string
}

func (e *TestError) Error() string {
	return e.Msg
}

func (e *TestError) Is(target error) bool {
	_, ok := target.(*TestError)
	return ok
}

func TestContext(t *testing.T) {
	// Wrap an error with context
	err := &TestError{Msg: "query error"}
	wrap := errors.WithContext{"key1": "value1"}.Wrap(err, "message")
	assert.NotNil(t, wrap)

	// Unwrap should return TestError
	u := errors.Unwrap(wrap)
	require.NotNil(t, u)
	assert.Equal(t, "query error", u.Error())

	// Extract as normal map
	m := errors.ToMap(wrap)
	require.NotNil(t, m)
	assert.Equal(t, "value1", m["key1"])

	// Can use errors.Is() from std `errors` package
	assert.True(t, errors.Is(err, &TestError{}))
	assert.True(t, errors.Is(wrap, &TestError{}))

	// Can use errors.As() from std `errors` package
	myErr := &TestError{}
	assert.True(t, errors.As(wrap, &myErr))
	assert.Equal(t, myErr.Msg, "query error")

	// Extract as Logrus fields
	f := errors.ToLogrus(wrap)
	require.NotNil(t, f)
	b := bytes.Buffer{}
	logrus.SetOutput(&b)
	logrus.WithFields(f).Info("test logrus fields")
	logrus.SetOutput(os.Stdout)
	assert.Contains(t, b.String(), "test logrus fields")
	assert.Contains(t, b.String(), `excValue="message: query error"`)
	assert.Contains(t, b.String(), `excType="*errors_test.TestError"`)
	assert.Contains(t, b.String(), "key1=value1")

	assert.Equal(t, "message: query error", wrap.Error())

	// Should output the message and fields
	out := fmt.Sprintf("%+v", wrap)
	assert.True(t, strings.Contains(out, `message: query error (key1=value1)`))
}

func TestWithErrorf(t *testing.T) {
	err := errors.New("this is an error")
	wrap := errors.WithContext{"key1": "value1", "key2": "value2"}.Wrap(err, "message")
	err = fmt.Errorf("wrapped: %w", wrap)

	out := fmt.Sprintf("final: %s", err)
	assert.Contains(t, out, "final: wrapped: message: this is an error")
	assert.Contains(t, out, "key1=value1")
	assert.Contains(t, out, "key2=value2")
}

func TestNestedWithContext(t *testing.T) {
	err := errors.New("this is an error")
	err = errors.WithContext{"key1": "value1"}.Wrap(err, "message")
	err = errors.WithContext{"key2": "value2"}.Wrap(err, "message")

	m := errors.ToMap(err)
	assert.NotNil(t, m)
	assert.Equal(t, "value1", m["key1"])
	assert.Equal(t, "value2", m["key2"])

	f := errors.ToLogrus(err)
	require.NotNil(t, f)
	b := bytes.Buffer{}
	logrus.SetOutput(&b)
	logrus.WithFields(f).Info("test logrus fields")
	logrus.SetOutput(os.Stdout)
	assert.Contains(t, b.String(), "test logrus fields")
	assert.Contains(t, b.String(), "key1=value1")
	assert.Contains(t, b.String(), "key2=value2")
}

func TestFmtDirectives(t *testing.T) {
	err := errors.WithContext{"key1": "value1"}.Wrap(errors.New("error"), "")
	assert.Equal(t, "value: : error (key1=value1)", fmt.Sprintf("value: %v", err))
	assert.Equal(t, "value+: : error (key1=value1)", fmt.Sprintf("value+: %+v", err))
	assert.Equal(t, "type: *errors.withContext", fmt.Sprintf("type: %T", err))
}

func TestWithStack(t *testing.T) {
	err := errors.WithStack(io.EOF)

	var files []string
	var funcs []string
	if cast, ok := err.(callstack.HasStackTrace); ok {
		for _, frame := range cast.StackTrace() {
			files = append(files, fmt.Sprintf("%s", frame))
			funcs = append(funcs, fmt.Sprintf("%n", frame))
		}
	}
	assert.True(t, linq.From(files).Contains("functional_test.go"))
	assert.True(t, linq.From(funcs).Contains("TestWithStack"), funcs)
}

func TestWithStackUnWrap(t *testing.T) {
	err := errors.WithStack(&TestError{Msg: "query error"})
	err = fmt.Errorf("wrapped: %w", err)

	// Can use errors.Is() from std `errors` package
	assert.True(t, errors.Is(err, &TestError{}))

	// Can use errors.As() from std `errors` package
	myErr := &TestError{}
	assert.True(t, errors.As(err, &myErr))
	assert.Equal(t, myErr.Msg, "query error")
}

func TestWrapContextWithStack(t *testing.T) {
	err := errors.WithContext{"key1": "value1"}.Wrap(errors.WithStack(&TestError{Msg: "error"}), "context")

	myErr := &TestError{}
	assert.True(t, errors.Is(err, &TestError{}))
	assert.True(t, errors.As(err, &myErr))
	assert.Equal(t, myErr.Msg, "error")
	assert.Equal(t, "context: error", err.Error())

	// Extract the stack from the error chain
	var stack callstack.HasStackTrace
	// Extract the stack info if provided
	assert.True(t, errors.As(err, &stack))

	trace := stack.StackTrace()
	caller := callstack.GetLastFrame(trace)
	assert.Contains(t, fmt.Sprintf("%+v", stack), "errors/functional_test.go:144")
	assert.Equal(t, "errors_test.TestWrapContextWithStack", caller.Func)
	assert.Equal(t, 144, caller.LineNo)
}

func TestIs(t *testing.T) {
	target := errors.New("wrapped")

	tests := []struct {
		name   string
		target error
		err    error
	}{
		{
			name:   "std_wrap",
			target: target,
			err:    fmt.Errorf("some reason: %w", target),
		},
		{
			name:   "std_double_wrap",
			target: target,
			err:    fmt.Errorf("reason 1: %w", fmt.Errorf("reason 2: %w", target)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, errors.Is(tt.err, tt.target))
			assert.ErrorIs(t, tt.err, tt.target)
		})
	}
}

func TestWrapfNil(t *testing.T) {
	got := errors.WithContext{"some": "context"}.Wrapf(nil, "no '%d' error", 1)
	assert.Nil(t, got)
}

func TestWrapNil(t *testing.T) {
	got := errors.WithContext{"some": "context"}.Wrap(nil, "no error")
	assert.Nil(t, got)
}

func TestFormatWithStack(t *testing.T) {
	tests := []struct {
		err    error
		Name   string
		format string
		want   []string
	}{{
		Name:   "withStack() string",
		err:    errors.WithStack(io.EOF),
		format: "%s",
		want:   []string{"EOF"},
	}, {
		Name:   "withStack() value",
		err:    errors.WithStack(io.EOF),
		format: "%v",
		want:   []string{"EOF"},
	}, {
		Name:   "withStack() value plus",
		err:    errors.WithStack(io.EOF),
		format: "%+v",
		want: []string{
			"EOF",
			"github.com/mailgun/holster/v5/errors_test.TestFormatWithStack",
		},
	}, {
		Name:   "withStack(errors.New()) string",
		err:    errors.WithStack(errors.New("error")),
		format: "%s",
		want:   []string{"error"},
	}, {
		Name:   "withStack(errors.New()) value",
		err:    errors.WithStack(errors.New("error")),
		format: "%v",
		want:   []string{"error"},
	}, {
		Name:   "withStack(errors.New()) value plus",
		err:    errors.WithStack(errors.New("error")),
		format: "%+v",
		want: []string{
			"error",
			"github.com/mailgun/holster/v5/errors_test.TestFormatWithStack",
			"errors/functional_test.go:238",
		},
	}, {
		Name:   "errors.WithStack(errors.WithStack(io.EOF)) value plus",
		err:    errors.WithStack(errors.WithStack(io.EOF)),
		format: "%+v",
		want: []string{"EOF",
			"github.com/mailgun/holster/v5/errors_test.TestFormatWithStack",
			"github.com/mailgun/holster/v5/errors_test.TestFormatWithStack",
		},
	}, {
		Name:   "deeply nested stack",
		err:    errors.WithStack(errors.WithStack(fmt.Errorf("message: %w", io.EOF))),
		format: "%+v",
		want: []string{"EOF",
			"message",
			"github.com/mailgun/holster/v5/errors_test.TestFormatWithStack",
			"github.com/mailgun/holster/v5/errors_test.TestFormatWithStack",
			"github.com/mailgun/holster/v5/errors_test.TestFormatWithStack",
		},
	}, {
		Name:   "WithStack with fmt.Errorf()",
		err:    errors.WithStack(fmt.Errorf("error%d", 1)),
		format: "%+v",
		want: []string{"error1",
			"github.com/mailgun/holster/v5/errors_test.TestFormatWithStack",
			"github.com/mailgun/holster/v5/errors_test.TestFormatWithStack",
		},
	}}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got := fmt.Sprintf(tt.format, tt.err)
			for _, s := range tt.want {
				assert.Contains(t, got, s)
			}
		})
	}
}
