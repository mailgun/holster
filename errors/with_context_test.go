package errors_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"io"

	"github.com/Sirupsen/logrus"
	"github.com/ahmetalpbalkan/go-linq"
	"github.com/mailgun/holster/errors"
	"github.com/mailgun/holster/stack"
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
	var capture bytes.Buffer

	// Wrap an error with context
	err := &TestError{Msg: "query error"}
	wrap := errors.WithContext{"key1": "value1"}.Wrap(err, "message")
	c.Assert(wrap, NotNil)

	// Can extract fields for logrus
	log := logrus.New()
	log.Out = &capture
	log.WithFields(errors.ToLogrus(wrap)).Info("Info Message")
	//fmt.Println(capture.String())
	c.Assert(strings.Contains(capture.String(), `msg="Info Message"`), Equals, true)
	c.Assert(strings.Contains(capture.String(), `level=info`), Equals, true)
	c.Assert(strings.Contains(capture.String(), `key1=value1`), Equals, true)
	c.Assert(strings.Contains(capture.String(), `go-func="TestContext()"`), Equals, true)
	c.Assert(strings.Contains(capture.String(), `go-line=40`), Equals, true)
	c.Assert(strings.Contains(capture.String(), `go-src="with_context_test.go"`), Equals, true)
	c.Assert(strings.Contains(capture.String(), `(*WithContextTestSuite).TestContext()`), Equals, true)

	// Extract as normal map
	errMap := errors.ToMap(wrap)
	c.Assert(errMap, NotNil)
	c.Assert(errMap["key1"], Equals, "value1")
	c.Assert(errMap["go-func"], Equals, "TestContext()")
	c.Assert(errMap["go-line"], Equals, 40)
	c.Assert(errMap["go-src"], Equals, "with_context_test.go")
	c.Assert(strings.Contains(errMap["go-call-stack"].(string), "(*WithContextTestSuite).TestContext()"), Equals, true)

	// Also implements the causer interface
	err = errors.Cause(wrap).(*TestError)
	c.Assert(err.Msg, Equals, "query error")

	out := fmt.Sprintf("%s", wrap)
	c.Assert(out, Equals, "message: query error")

	// Should output the message and the fields
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
