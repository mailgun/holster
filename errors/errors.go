package errors

import (
	"errors"
	"fmt"
)

// Import all the standard errors functions as a convenience.

// Is reports whether any error in err's chain matches target.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// Errorf is deprecated use fmt.Errorf instead
func Errorf(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

// As finds the first error in err's chain that matches target, and if so, sets
// target to that error value and returns true.
func As(err error, target any) bool {
	return errors.As(err, target)
}

// New returns an error that formats as the given text.
// Each call to New returns a distinct error value even if the text is identical.
func New(text string) error {
	return errors.New(text)
}

// Unwrap returns the result of calling the Unwrap method on err, if err's
// type contains an Unwrap method returning error.
// Otherwise, Unwrap returns nil.
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// Wrap is deprecated use fmt.Errorf() instead
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// Wrapf is deprecated use fmt.Errorf() instead
func Wrapf(err error, msg string, a ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(msg, a...), err)
}
