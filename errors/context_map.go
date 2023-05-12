package errors

import (
	"bytes"
	"fmt"
	"io"

	"github.com/mailgun/holster/v4/callstack"
	pkgerrors "github.com/pkg/errors" //nolint:depguard // Legacy code requires deprecated package.
)

// Implements the `error` `causer` and `Contexter` interfaces
type withContext struct {
	context WithContext
	msg     string
	cause   error
	stack   *callstack.CallStack
}

func (c *withContext) Cause() error {
	return c.cause
}

func (c *withContext) Error() string {
	if c.msg == "" {
		return c.cause.Error()
	}
	return c.msg + ": " + c.cause.Error()
}

func (c *withContext) StackTrace() pkgerrors.StackTrace {
	if child, ok := c.cause.(callstack.HasStackTrace); ok {
		return child.StackTrace()
	}
	return c.stack.StackTrace()
}

func (c *withContext) Context() map[string]interface{} {
	result := make(map[string]interface{}, len(c.context))
	for key, value := range c.context {
		result[key] = value
	}

	// downstream context values have precedence as they are closer to the cause
	if child, ok := c.cause.(HasContext); ok {
		downstream := child.Context()
		if downstream == nil {
			return result
		}

		for key, value := range downstream {
			result[key] = value
		}
	}
	return result
}

func (c *withContext) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		_, _ = fmt.Fprintf(s, "%s: %+v (%s)", c.msg, c.Cause(), c.FormatFields())
	case 's', 'q':
		_, _ = io.WriteString(s, c.Error())
	}
}

func (c *withContext) FormatFields() string {
	var buf bytes.Buffer
	var count int

	for key, value := range c.context {
		if count > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(fmt.Sprintf("%+v=%+v", key, value))
		count++
	}
	return buf.String()
}

func (c *withContext) Unwrap() error {
	return c.cause
}
