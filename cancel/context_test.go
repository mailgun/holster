package cancel_test

import (
	"context"
	"testing"
	"time"

	"github.com/mailgun/holster/v4/cancel"
)

func TestWrapFirst(t *testing.T) {
	// First context
	firstCtx := cancel.New(context.Background())
	// Second context
	secondCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Now if either firstCtx or secondCtx is cancelled 'ctx' should cancel
	ctx := firstCtx.Wrap(secondCtx)

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	firstCtx.Cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for context to cancel")
	}
}

func TestWrapSecond(t *testing.T) {
	// First context
	firstCtx := cancel.New(context.Background())
	// Second context
	secondCtx, cancel := context.WithCancel(context.Background())

	// Now if either firstCtx or secondCtx is cancelled 'ctx' should cancel
	ctx := firstCtx.Wrap(secondCtx)

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for context to cancel")
	}
}
