package ctxutil_test

import (
	"context"
	"testing"
	"time"

	"github.com/mailgun/holster/v4/ctxutil"
	"github.com/stretchr/testify/require"
)

func TestWithDeadline(t *testing.T) {
	t.Run("Happy path", func(t *testing.T) {
		ctx := context.Background()
		deadline := time.Now().Add(1 * time.Hour)
		ctx2, cancel := ctxutil.WithDeadline(ctx, deadline)
		cancel()
		require.NotNil(t, ctx2)
	})
}

func TestWithTimeout(t *testing.T) {
	t.Run("Happy path", func(t *testing.T) {
		ctx := context.Background()
		timeout := 1 * time.Hour
		ctx2, cancel := ctxutil.WithTimeout(ctx, timeout)
		cancel()
		require.NotNil(t, ctx2)
	})
}
