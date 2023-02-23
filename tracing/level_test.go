package tracing_test

import (
	"strings"
	"testing"

	"github.com/mailgun/holster/v4/tracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLevel(t *testing.T) {
	t.Run("String()", func(t *testing.T) {
		assert.Equal(t, tracing.PanicLevel.String(), "PANIC")
		assert.Equal(t, tracing.FatalLevel.String(), "FATAL")
		assert.Equal(t, tracing.ErrorLevel.String(), "ERROR")
		assert.Equal(t, tracing.WarnLevel.String(), "WARNING")
		assert.Equal(t, tracing.InfoLevel.String(), "INFO")
		assert.Equal(t, tracing.DebugLevel.String(), "DEBUG")
		assert.Equal(t, tracing.TraceLevel.String(), "TRACE")
	})

	t.Run("ParseLogLevel()", func(t *testing.T) {
		testCases := []struct {
			Input    string
			Expected tracing.Level
		}{
			{Input: "PANIC", Expected: tracing.PanicLevel},
			{Input: "FATAL", Expected: tracing.FatalLevel},
			{Input: "ERROR", Expected: tracing.ErrorLevel},
			{Input: "WARNING", Expected: tracing.WarnLevel},
			{Input: "INFO", Expected: tracing.InfoLevel},
			{Input: "DEBUG", Expected: tracing.DebugLevel},
			{Input: "TRACE", Expected: tracing.TraceLevel},
		}

		for _, testCase := range testCases {
			t.Run(testCase.Input, func(t *testing.T) {
				level, err := tracing.ParseLogLevel(testCase.Input)
				require.NoError(t, err)
				assert.Equal(t, testCase.Expected, level)
				level, err = tracing.ParseLogLevel(strings.ToLower(testCase.Input))
				require.NoError(t, err)
				assert.Equal(t, testCase.Expected, level)
			})
		}

		t.Run("Handle parse error", func(t *testing.T) {
			_, err := tracing.ParseLogLevel("bogus")
			assert.ErrorContains(t, err, "unknown log level")
		})
	})
}
