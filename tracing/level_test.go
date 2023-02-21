package tracing_test

import (
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
		level, err := tracing.ParseLogLevel("PANIC")
		require.NoError(t, err)
		assert.Equal(t, tracing.PanicLevel, level)
		level, err = tracing.ParseLogLevel("FATAL")
		require.NoError(t, err)
		assert.Equal(t, tracing.FatalLevel, level)
		level, err = tracing.ParseLogLevel("ERROR")
		require.NoError(t, err)
		assert.Equal(t, tracing.ErrorLevel, level)
		level, err = tracing.ParseLogLevel("WARNING")
		require.NoError(t, err)
		assert.Equal(t, tracing.WarnLevel, level)
		level, err = tracing.ParseLogLevel("INFO")
		require.NoError(t, err)
		assert.Equal(t, tracing.InfoLevel, level)
		level, err = tracing.ParseLogLevel("DEBUG")
		require.NoError(t, err)
		assert.Equal(t, tracing.DebugLevel, level)
		level, err = tracing.ParseLogLevel("TRACE")
		require.NoError(t, err)
		assert.Equal(t, tracing.TraceLevel, level)
	})
}
