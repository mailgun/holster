package errors

import (
	"fmt"

	"github.com/mailgun/holster/stack"
)

// Implement this interface to pass along unstructured context to the logger
type HasContext interface {
	Context() map[string]interface{}
}

// Creates errors that conform to the `Contexter` interface
type WithContext map[string]interface{}

func (c WithContext) Wrapf(err error, format string, args ...interface{}) error {
	return &withContext{
		stack:   stack.New(1),
		context: c,
		cause:   err,
		msg:     fmt.Sprintf(format, args...),
	}
}

func (c WithContext) Wrap(err error, msg string) error {
	return &withContext{
		stack:   stack.New(1),
		context: c,
		cause:   err,
		msg:     msg,
	}
}
