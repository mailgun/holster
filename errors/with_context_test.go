package errors_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/ahmetb/go-linq"
	"github.com/mailgun/holster/v3/errors"
	"github.com/mailgun/holster/v3/stack"
	. "gopkg.in/check.v1"
)

type TestError struct {
	Msg string
}

func (err *TestError) Error() string {
	return err.Msg
}

func TestErrors(t *testing.T) { TestingT(t) }

type WithContextTestSuite struct{}

var _ = Suite(&WithContextTestSuite{})

func (s *WithContextTestSuite) SetUpSuite(c *C) {
}

func (s *WithContextTestSuite) TestContext(c *C) {
	// Wrap an error with context
	err := &TestError{Msg: "query error"}
	wrap := errors.WithContext{"key1": "value1"}.Wrap(err, "message")
	c.Assert(wrap, NotNil)

	// Extract as normal map
	errMap := errors.ToMap(wrap)
	c.Assert(errMap, NotNil)
	c.Assert(errMap["key1"], Equals, "value1")

	// Also implements the causer interface
	err = errors.Cause(wrap).(*TestError)
	c.Assert(err.Msg, Equals, "query error")

	out := fmt.Sprintf("%s", wrap)
	c.Assert(out, Equals, "message: query error")

	// Should output the message, fields and trace
	out = fmt.Sprintf("%+v", wrap)
	c.Assert(strings.Contains(out, `message: query error (`), Equals, true)
	c.Assert(strings.Contains(out, `key1=value1`), Equals, true)
}

func (s *WithContextTestSuite) TestWithStack(c *C) {
	err := errors.WithStack(io.EOF)

	var files []string
	var funcs []string
	if cast, ok := err.(stack.HasStackTrace); ok {
		for _, frame := range cast.StackTrace() {
			files = append(files, fmt.Sprintf("%s", frame))
			funcs = append(funcs, fmt.Sprintf("%n", frame))
		}
	}
	c.Assert(linq.From(files).Contains("with_context_test.go"), Equals, true)
	c.Assert(linq.From(funcs).Contains("(*WithContextTestSuite).TestWithStack"), Equals, true)
}

func (s *WithContextTestSuite) TestWrapfNil(c *C) {
	got := errors.WithContext{"some": "context"}.Wrapf(nil, "no error")
	c.Assert(got, IsNil)
}

func (s *WithContextTestSuite) TestWrapNil(c *C) {
	got := errors.WithContext{"some": "context"}.Wrap(nil, "no error")
	c.Assert(got, IsNil)
}
