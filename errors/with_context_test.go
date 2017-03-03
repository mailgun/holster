package errors_test

import (
	"bytes"
	"testing"

	"fmt"

	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/mailgun/holster/errors"
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
	c.Assert(strings.Contains(capture.String(), `level=info msg="Info Message"`), Equals, true)
	c.Assert(strings.Contains(capture.String(), `key1=value1`), Equals, true)
	c.Assert(strings.Contains(capture.String(), `go-func="TestContext()"`), Equals, true)
	c.Assert(strings.Contains(capture.String(), `go-line=38`), Equals, true)
	c.Assert(strings.Contains(capture.String(), `go-src="with_context_test.go"`), Equals, true)

	// Extract as normal map
	errMap := errors.ToMap(wrap)
	c.Assert(errMap, NotNil)
	c.Assert(errMap, DeepEquals, map[string]interface{}{
		"key1":    "value1",
		"go-func": "TestContext()",
		"go-line": 38,
		"go-src":  "with_context_test.go",
	})

	// Also implements the causer interface
	err = errors.Cause(wrap).(*TestError)
	c.Assert(err.Msg, Equals, "query error")

	out := fmt.Sprintf("%s", wrap)
	c.Assert(out, Equals, "message: query error")

	// Should output the message and the fields
	out = fmt.Sprintf("%+v", wrap)
	c.Assert(strings.Contains(out, `message: query error (`), Equals, true)
	c.Assert(strings.Contains(out, `key1=value1`), Equals, true)
	c.Assert(strings.Contains(out, `go-func=TestContext()`), Equals, true)
	c.Assert(strings.Contains(out, `go-line=38`), Equals, true)
	c.Assert(strings.Contains(out, `go-src=with_context_test.go`), Equals, true)
}
