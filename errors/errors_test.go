package errors_test

import (
	goerrors "errors"
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/mailgun/holster/v4/errors"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	tests := []struct {
		err  string
		want error
	}{
		{"", fmt.Errorf("")},
		{"foo", fmt.Errorf("foo")},
		{"foo", errors.New("foo")},
		{"string with format specifiers: %v", goerrors.New("string with format specifiers: %v")},
	}

	for _, tt := range tests {
		got := errors.New(tt.err)
		if got.Error() != tt.want.Error() {
			t.Errorf("New.Error(): got: %q, want %q", got, tt.want)
		}
	}
}

func TestWrapNil(t *testing.T) {
	got := errors.Wrap(nil, "no error")
	if got != nil {
		t.Errorf("Wrap(nil, \"no error\"): got %#v, expected nil", got)
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		err     error
		message string
		want    string
	}{
		{io.EOF, "read error", "read error: EOF"},
		{errors.Wrap(io.EOF, "read error"), "client error", "client error: read error: EOF"},
	}

	for _, tt := range tests {
		got := errors.Wrap(tt.err, tt.message).Error()
		if got != tt.want {
			t.Errorf("Wrap(%v, %q): got: %v, want %v", tt.err, tt.message, got, tt.want)
		}
	}
}

type nilError struct{}

func (nilError) Error() string { return "nil error" }

func TestCause(t *testing.T) {
	x := errors.New("error")
	tests := []struct {
		err  error
		want error
	}{{
		// nil error is nil
		err:  nil,
		want: nil,
	}, {
		// explicit nil error is nil
		err:  (error)(nil),
		want: nil,
	}, {
		// typed nil is nil
		err:  (*nilError)(nil),
		want: (*nilError)(nil),
	}, {
		// uncaused error is unaffected
		err:  io.EOF,
		want: io.EOF,
	}, {
		// caused error returns cause
		err:  errors.Wrap(io.EOF, "ignored"),
		want: io.EOF,
	}, {
		err:  x, // return from errors.New
		want: x,
	}, {
		errors.WithMessage(nil, "whoops"),
		nil,
	}, {
		errors.WithMessage(io.EOF, "whoops"),
		io.EOF,
	}, {
		errors.WithStack(nil),
		nil,
	}, {
		errors.WithStack(io.EOF),
		io.EOF,
	}}

	for i, tt := range tests {
		got := errors.Cause(tt.err)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("test %d: got %#v, want %#v", i+1, got, tt.want)
		}
	}
}

func TestWrapfNil(t *testing.T) {
	got := errors.Wrapf(nil, "no error")
	if got != nil {
		t.Errorf("Wrapf(nil, \"no error\"): got %#v, expected nil", got)
	}
}

func TestWrapf(t *testing.T) {
	tests := []struct {
		err     error
		message string
		want    string
	}{
		{io.EOF, "read error", "read error: EOF"},
		{errors.Wrapf(io.EOF, "read error without format specifiers"), "client error", "client error: read error without format specifiers: EOF"},
		{errors.Wrapf(io.EOF, "read error with %d format specifier", 1), "client error", "client error: read error with 1 format specifier: EOF"},
	}

	for _, tt := range tests {
		got := errors.Wrapf(tt.err, tt.message).Error()
		if got != tt.want {
			t.Errorf("Wrapf(%v, %q): got: %v, want %v", tt.err, tt.message, got, tt.want)
		}
	}
}

func TestErrorf(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{errors.Errorf("read error without format specifiers"), "read error without format specifiers"},
		{errors.Errorf("read error with %d format specifier", 1), "read error with 1 format specifier"},
	}

	for _, tt := range tests {
		got := tt.err.Error()
		if got != tt.want {
			t.Errorf("Errorf(%v): got: %q, want %q", tt.err, got, tt.want)
		}
	}
}

func TestWithStackNil(t *testing.T) {
	got := errors.WithStack(nil)
	if got != nil {
		t.Errorf("WithStack(nil): got %#v, expected nil", got)
	}
}

func TestWithStack(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{io.EOF, "EOF"},
		{errors.WithStack(io.EOF), "EOF"},
	}

	for _, tt := range tests {
		got := errors.WithStack(tt.err).Error()
		if got != tt.want {
			t.Errorf("WithStack(%v): got: %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestWithMessageNil(t *testing.T) {
	got := errors.WithMessage(nil, "no error")
	if got != nil {
		t.Errorf("WithMessage(nil, \"no error\"): got %#v, expected nil", got)
	}
}

func TestWithMessage(t *testing.T) {
	tests := []struct {
		err     error
		message string
		want    string
	}{
		{io.EOF, "read error", "read error: EOF"},
		{errors.WithMessage(io.EOF, "read error"), "client error", "client error: read error: EOF"},
	}

	for _, tt := range tests {
		got := errors.WithMessage(tt.err, tt.message).Error()
		if got != tt.want {
			t.Errorf("WithMessage(%v, %q): got: %q, want %q", tt.err, tt.message, got, tt.want)
		}
	}
}

// errors.New, etc values are not expected to be compared by value
// but the change in errors#27 made them incomparable. Assert that
// various kinds of errors have a functional equality operator, even
// if the result of that equality is always false.
func TestErrorEquality(t *testing.T) {
	vals := []error{
		nil,
		io.EOF,
		goerrors.New("EOF"),
		errors.New("EOF"),
		errors.Errorf("EOF"),
		errors.Wrap(io.EOF, "EOF"),
		errors.Wrapf(io.EOF, "EOF%d", 2),
		errors.WithMessage(nil, "whoops"),
		errors.WithMessage(io.EOF, "whoops"),
		errors.WithStack(io.EOF),
		errors.WithStack(nil),
	}

	for i := range vals {
		for j := range vals {
			_ = vals[i] == vals[j] // mustn't panic
		}
	}
}

func TestTypedError(t *testing.T) {
	const ErrClass = "FoobarClass"
	const ErrType = "FoobarType"

	t.Run("NewWithType()", func(t *testing.T) {
		err := errors.NewWithType("Foobar", ErrClass, ErrType)
		assert.Equal(t, "Foobar", err.Error())
		assert.Equal(t, ErrClass, err.Class())
		assert.Equal(t, ErrType, err.Type())
	})

	t.Run("WrapWithType()", func(t *testing.T) {
		err := errors.New("Foobar")
		err2 := errors.WrapWithType(err, ErrClass, ErrType)
		assert.Equal(t, "Foobar", err2.Error())
		assert.Equal(t, ErrClass, err2.Class())
		assert.Equal(t, ErrType, err2.Type())
	})
}
