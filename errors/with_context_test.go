package errors_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	linq "github.com/ahmetb/go-linq"
	"github.com/mailgun/holster/v4/callstack"
	"github.com/mailgun/holster/v4/errors"
	"github.com/stretchr/testify/assert"
)

type TestError struct {
	Msg string
}

func (err *TestError) Error() string {
	return err.Msg
}

func TestContext(t *testing.T) {
	// Wrap an error with context
	err := &TestError{Msg: "query error"}
	wrap := errors.WithContext{"key1": "value1"}.Wrap(err, "message")
	assert.NotNil(t, wrap)

	// Extract as normal map
	errMap := errors.ToMap(wrap)
	assert.NotNil(t, errMap)
	assert.Equal(t, "value1", errMap["key1"])

	// Also implements the causer interface
	err = errors.Cause(wrap).(*TestError)
	assert.Equal(t, "query error", err.Msg)

	out := wrap.Error()
	assert.Equal(t, "message: query error", out)

	// Should output the message, fields and trace
	out = fmt.Sprintf("%+v", wrap)
	assert.True(t, strings.Contains(out, `message: query error (`))
	assert.True(t, strings.Contains(out, `key1=value1`))
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
	assert.True(t, linq.From(files).Contains("with_context_test.go"))
	assert.True(t, linq.From(funcs).Contains("TestWithStack"), funcs)
}

func TestWrapfNil(t *testing.T) {
	got := errors.WithContext{"some": "context"}.Wrapf(nil, "no error")
	assert.Nil(t, got)
}

func TestWrapNil(t *testing.T) {
	got := errors.WithContext{"some": "context"}.Wrap(nil, "no error")
	assert.Nil(t, got)
}
