package errors

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/mailgun/holster/v5/callstack"
	"github.com/sirupsen/logrus"
)

// HasContext Implement this interface to pass along unstructured context to the logger
type HasContext interface {
	Context() map[string]interface{}
}

// HasFormat True if the interface has the format method (from fmt package)
type HasFormat interface {
	Format(st fmt.State, verb rune)
}

// WithContext Creates errors that conform to the `HasContext` interface
type WithContext map[string]interface{}

// Wrapf returns an error annotating err with a stack trace
// at the point Wrapf is call, and the format specifier.
// If err is nil, Wrapf returns nil.
func (c WithContext) Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return &withContext{
		stack:   callstack.New(1),
		context: c,
		cause:   err,
		msg:     fmt.Sprintf(format, args...),
	}
}

// Wrap returns an error annotating err with a stack trace
// at the point Wrap is called, and the supplied message.
// If err is nil, Wrap returns nil.
func (c WithContext) Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &withContext{
		stack:   callstack.New(1),
		context: c,
		cause:   err,
		msg:     msg,
	}
}

func (c WithContext) Error(msg string) error {
	return &withContext{
		stack:   callstack.New(1),
		context: c,
		cause:   errors.New(msg),
		msg:     "",
	}
}

func (c WithContext) Errorf(format string, args ...interface{}) error {
	return &withContext{
		stack:   callstack.New(1),
		context: c,
		cause:   fmt.Errorf(format, args...),
		msg:     "",
	}
}

// Implements the `error` `causer` and `Contexter` interfaces
type withContext struct {
	context WithContext
	msg     string
	cause   error
	stack   *callstack.CallStack
}

func (c *withContext) Unwrap() error {
	return c.cause
}

func (c *withContext) Is(target error) bool {
	_, ok := target.(*withContext)
	return ok
}

func (c *withContext) Error() string {
	if c.msg == "" {
		return c.cause.Error()
	}
	return c.msg + ": " + c.cause.Error()
}

func (c *withContext) StackTrace() callstack.StackTrace {
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
		_, _ = fmt.Fprintf(s, "%s: %+v (%s)", c.msg, c.Unwrap(), c.FormatFields())
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

// ToMap Returns the context for the underlying error as map[string]interface{}
// If no context is available returns nil
func ToMap(err error) map[string]interface{} {
	var result map[string]interface{}

	if child, ok := err.(HasContext); ok {
		// Append the context map to our results
		result = make(map[string]interface{})
		for key, value := range child.Context() {
			result[key] = value
		}
	}
	return result
}

// ToLogrus Returns the context and stacktrace information for the underlying error as logrus.Fields{}
// returns empty logrus.Fields{} if err has no context or no stacktrace
//
//	logrus.WithFields(errors.ToLogrus(err)).WithField("tid", 1).Error(err)
func ToLogrus(err error) logrus.Fields {
	result := logrus.Fields{
		"excValue": err.Error(),
		"excType":  fmt.Sprintf("%T", Unwrap(err)),
	}

	// Add the stack info if provided
	if cast, ok := err.(callstack.HasStackTrace); ok {
		trace := cast.StackTrace()
		caller := callstack.GetLastFrame(trace)
		result["excFuncName"] = caller.Func
		result["excLineno"] = caller.LineNo
		result["excFileName"] = caller.File
	}

	// Add context if provided
	child, ok := err.(HasContext)
	if !ok {
		return result
	}

	// Append the context map to our results
	for key, value := range child.Context() {
		result[key] = value
	}
	return result
}
