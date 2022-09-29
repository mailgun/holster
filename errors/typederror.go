package errors

// TypedError is used to decorate an error with classification metadata.
// Contains error class and type decoration.
type TypedError struct {
	error
	cls string
	typ string
}

// NewWithType returns an error decorated with error class and type.
// Best practice: use public constants for class and type.
func NewWithType(msg, cls, typ string) *TypedError {
	return &TypedError{
		error: New(msg),
		cls:   cls,
		typ:   typ,
	}
}

// WrapWithType returns a wrapped error decorated with class and type.
// Class is a category of error types.
// Type is the specific error condition.
// Best practice: use public constants for class and type.
func WrapWithType(err error, cls, typ string) *TypedError {
	return &TypedError{
		error: err,
		cls:   cls,
		typ:   typ,
	}
}

func (e *TypedError) Class() string {
	return e.cls
}

func (e *TypedError) Type() string {
	return e.typ
}
