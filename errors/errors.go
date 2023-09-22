// Package errors provides simple error handling primitives.
//
// The traditional error handling idiom in Go is roughly akin to
//
//	if err != nil {
//	        return err
//	}
//
// which applied recursively up the call stack results in error reports
// without context or debugging information. The errors package allows
// programmers to add context to the failure path in their code in a way
// that does not destroy the original value of the error.
//
// # Adding context to an error
//
// The errors.Wrap function returns a new error that adds context to the
// original error by recording a stack trace at the point Wrap is called,
// and the supplied message. For example
//
//	_, err := ioutil.ReadAll(r)
//	if err != nil {
//	        return errors.Wrap(err, "read failed")
//	}
//
// If additional control is required the errors.WithStack and errors.WithMessage
// functions destructure errors.Wrap into its component operations of annotating
// an error with a stack trace and an a message, respectively.
//
// # Retrieving the cause of an error
//
// Using errors.Wrap constructs a stack of errors, adding context to the
// preceding error. Depending on the nature of the error it may be necessary
// to reverse the operation of errors.Wrap to retrieve the original error
// for inspection. Any error value which implements this interface
//
//	type causer interface {
//	        Cause() error
//	}
//
// can be inspected by errors.Cause. errors.Cause will recursively retrieve
// the topmost error which does not implement causer, which is assumed to be
// the original cause. For example:
//
//	switch err := errors.Cause(err).(type) {
//	case *MyError:
//	        // handle specifically
//	default:
//	        // unknown error
//	}
//
// causer interface is not exported by this package, but is considered a part
// of stable public API.
//
// # Formatted printing of errors
//
// All error values returned from this package implement fmt.Formatter and can
// be formatted by the fmt package. The following verbs are supported
//
//	%s    print the error. If the error has a Cause it will be
//	      printed recursively
//	%v    see %s
//	%+v   extended format. Each Frame of the error's StackTrace will
//	      be printed in detail.
//
// # Retrieving the stack trace of an error or wrapper
//
// New, Errorf, Wrap, and Wrapf record a stack trace at the point they are
// invoked. This information can be retrieved with the following interface.
//
//	type stackTracer interface {
//	        StackTrace() errors.StackTrace
//	}
//
// Where errors.StackTrace is defined as
//
//	type StackTrace []Frame
//
// The Frame type represents a call site in the stack trace. Frame supports
// the fmt.Formatter interface that can be used for printing information about
// the stack trace of this error. For example:
//
//	if err, ok := err.(stackTracer); ok {
//	        for _, f := range err.StackTrace() {
//	                fmt.Printf("%+s:%d", f)
//	        }
//	}
//
// stackTracer interface is not exported by this package, but is considered a part
// of stable public API.
//
// See the documentation for Frame.Format for more details.
package errors

import (
	"fmt"
	"io"

	stack "github.com/mailgun/holster/v4/callstack"
	pkgerrors "github.com/pkg/errors" //nolint:gomodguard // Legacy code requires deprecated package.
	"github.com/sirupsen/logrus"
)

// New returns an error with the supplied message.
// New also records the stack trace at the depth specified.
func NewWithDepth(message string, depth int) error {
	return &fundamental{
		msg:       message,
		CallStack: stack.New(depth),
	}
}

// New returns an error with the supplied message.
// New also records the stack trace at the point it was called.
func New(message string) error {
	return &fundamental{
		msg:       message,
		CallStack: stack.New(1),
	}
}

// Errorf formats according to a format specifier and returns the string
// as a value that satisfies error.
// Errorf also records the stack trace at the point it was called.
func Errorf(format string, args ...interface{}) error {
	return &fundamental{
		msg:       fmt.Sprintf(format, args...),
		CallStack: stack.New(1),
	}
}

// fundamental is an error that has a message and a stack, but no caller.
type fundamental struct {
	*stack.CallStack
	msg string
}

func (f *fundamental) Error() string { return f.msg }

func (f *fundamental) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = io.WriteString(s, f.msg)
			f.CallStack.Format(s, verb)
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, f.msg)
	case 'q':
		fmt.Fprintf(s, "%q", f.msg)
	}
}

// WithStack annotates err with a stack trace at the point WithStack was called.
// If err is nil, WithStack returns nil.
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	return &withStack{
		err,
		stack.New(1),
	}
}

type withStack struct {
	error
	*stack.CallStack
}

func (w *withStack) Cause() error  { return w.error }
func (w *withStack) Unwrap() error { return w.error }
func (w *withStack) Context() map[string]interface{} {
	if child, ok := w.error.(HasContext); ok {
		return child.Context()
	}
	return nil
}

func (w *withStack) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v", w.Cause())
			w.CallStack.Format(s, verb)
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, w.Error())
	case 'q':
		fmt.Fprintf(s, "%q", w.Error())
	}
}

// WrapStack returns an error annotating err with a stack trace
// at the depth indicated. A calling depth of 1 is the function that
// called WrapStack. If err is nil, Wrap returns nil.
func WrapWithDepth(err error, message string, depth int) error {
	if err == nil {
		return nil
	}
	err = &withMessage{
		cause: err,
		msg:   message,
	}
	return &withStack{
		err,
		stack.New(depth),
	}
}

// Wrap returns an error annotating err with a stack trace
// at the point Wrap is called, and the supplied message.
// If err is nil, Wrap returns nil.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	err = &withMessage{
		cause: err,
		msg:   message,
	}
	return &withStack{
		err,
		stack.New(1),
	}
}

// Wrapf returns an error annotating err with a stack trace
// at the point Wrapf is call, and the format specifier.
// If err is nil, Wrapf returns nil.
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	err = &withMessage{
		cause: err,
		msg:   fmt.Sprintf(format, args...),
	}
	return &withStack{
		err,
		stack.New(1),
	}
}

// WithMessage annotates err with a new message.
// If err is nil, WithMessage returns nil.
func WithMessage(err error, message string) error {
	if err == nil {
		return nil
	}
	return &withMessage{
		cause: err,
		msg:   message,
	}
}

type withMessage struct {
	cause error
	msg   string
}

func (w *withMessage) Error() string { return w.msg + ": " + w.cause.Error() }
func (w *withMessage) Cause() error  { return w.cause }
func (w *withMessage) Unwrap() error { return w.cause }

func (w *withMessage) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v\n", w.Cause())
			_, _ = io.WriteString(s, w.msg)
			return
		}
		fallthrough
	case 's', 'q':
		_, _ = io.WriteString(s, w.Error())
	}
}

// Cause returns the underlying cause of the error, if possible.
// An error value has a cause if it implements the following
// interface:
//
//	type causer interface {
//	       Cause() error
//	}
//
// If the error does not implement Cause, the original error will
// be returned. If the error is nil, nil will be returned without further
// investigation.
func Cause(err error) error {
	type causer interface {
		Cause() error
	}

	for err != nil {
		cause, ok := err.(causer)
		if !ok {
			break
		}
		err = cause.Cause()
	}
	return err
}

// Returns the context for the underlying error as map[string]interface{}
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

// Returns the context and stacktrace information for the underlying error as logrus.Fields{}
// returns empty logrus.Fields{} if err has no context or no stacktrace
//
//	logrus.WithFields(errors.ToLogrus(err)).WithField("tid", 1).Error(err)
func ToLogrus(err error) logrus.Fields {
	result := logrus.Fields{
		"excValue": err.Error(),
		"excType":  fmt.Sprintf("%T", Cause(err)),
		"excText":  fmt.Sprintf("%+v", err),
	}

	// Add the stack info if provided
	if cast, ok := err.(stack.HasStackTrace); ok {
		trace := cast.StackTrace()
		caller := stack.GetLastFrame(trace)
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

type CauseError struct {
	stack *stack.CallStack
	error error
}

// Creates a new error that becomes the cause even if 'err' is a wrapped error
// but preserves the Context() and StackTrace() information. This allows the user
// to create a concrete error type without losing context
//
//	// Our new concrete type encapsulates CauseError
//	type RetryError struct {
//		errors.CauseError
//	}
//
//	func NewRetryError(err error) *RetryError {
//		return &RetryError{errors.NewCauseError(err, 1)}
//	}
//
//	// Returns true if the error is of type RetryError
//	func IsRetryError(err error) bool {
//		err = errors.Cause(err)
//		_, ok := err.(*RetryError)
//		return ok
//	}
func NewCauseError(err error, depth ...int) *CauseError {
	var stk *stack.CallStack
	if len(depth) > 0 {
		stk = stack.New(1 + depth[0])
	} else {
		stk = stack.New(1)
	}
	return &CauseError{
		stack: stk,
		error: err,
	}
}

func (e *CauseError) Error() string { return e.error.Error() }
func (e *CauseError) Context() map[string]interface{} {
	if child, ok := e.error.(HasContext); ok {
		return child.Context()
	}
	return nil
}
func (e *CauseError) StackTrace() pkgerrors.StackTrace {
	if child, ok := e.error.(stack.HasStackTrace); ok {
		return child.StackTrace()
	}
	return e.stack.StackTrace()
}

// TODO: Add Format() support
