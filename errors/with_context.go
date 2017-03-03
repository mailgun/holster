package errors

import "fmt"

// Implement this interface to pass along unstructured context to the logger
type HasContext interface {
	Context() map[string]interface{}
}

// Creates errors that conform to the `Contexter` interface
type WithContext map[string]interface{}

func (c WithContext) Wrapf(err error, format string, args ...interface{}) error {
	caller := GetCaller(1)
	return &ContextMap{
		cause: err,
		context: union(c, WithContext{
			"go-func": caller.Func,
			"go-line": caller.LineNo,
			"go-src":  caller.File,
		}),
		msg: fmt.Sprintf(format, args...),
	}
}

func (c WithContext) Wrap(err error, msg string) error {
	caller := GetCaller(1)
	return &ContextMap{
		cause: err,
		context: union(c, WithContext{
			"go-func": caller.Func,
			"go-line": caller.LineNo,
			"go-src":  caller.File,
		}),
		msg: msg,
	}
}

// Return a new Union of the two maps
func union(lvalue, rvalue WithContext) WithContext {
	// result is possibly larger than needed, but that should be ok
	result := make(WithContext, len(lvalue)+len(rvalue))
	for key, value := range lvalue {
		result[key] = value
	}
	for key, value := range rvalue {
		result[key] = value
	}
	return result
}
