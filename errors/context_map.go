package errors

import (
	"bytes"
	"fmt"
	"io"
)

// Implements the `error` `causer` and `Contexter` interfaces
type ContextMap struct {
	context WithContext
	msg     string
	cause   error
}

func (c *ContextMap) Error() string { return c.msg + ": " + c.cause.Error() }
func (c *ContextMap) Cause() error  { return c.cause }

func (c *ContextMap) Context() map[string]interface{} {
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

func (c *ContextMap) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%s: %+v (%s)", c.msg, c.Cause(), c.FormatFields())
			return
		}
		fallthrough
	case 's', 'q':
		io.WriteString(s, c.Error())
	}
}

func (c *ContextMap) FormatFields() string {
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
