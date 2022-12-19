package tracing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getCallerSpanName(t *testing.T) {
	gotSpanName, _ := getCallerSpanName(0)
	assert.Equal(t, "github.com/mailgun/holster/v4/tracing.Test_getCallerSpanName", gotSpanName)

	gotSpanName = NestedCaller()
	assert.Equal(t, "github.com/mailgun/holster/v4/tracing.Test_getCallerSpanName", gotSpanName)
}

func NestedCaller() (spanName string) {
	spanName, _ = getCallerSpanName(1)

	return spanName
}
