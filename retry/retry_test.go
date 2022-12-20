package retry_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mailgun/holster/v4/errors"
	"github.com/mailgun/holster/v4/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errCause = errors.New("cause of error")

func TestUntilInterval(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
	defer cancel()
	err := retry.Until(ctx, retry.Interval(time.Millisecond*10), func(ctx context.Context, att int) error {
		return errCause
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, &retry.Err{}))

	// Inspect the error
	var retryErr *retry.Err
	assert.True(t, errors.As(err, &retryErr))
	assert.GreaterOrEqual(t, retryErr.Attempts, 18)
	assert.LessOrEqual(t, retryErr.Attempts, 19)
	assert.Equal(t, retry.Cancelled, retryErr.Reason)

	// Cause() works as expected
	cause := errors.Cause(err)
	assert.Equal(t, errCause, cause)
}

func TestUntilNoError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
	defer cancel()
	err := retry.Until(ctx, retry.Interval(time.Millisecond*10), func(ctx context.Context, att int) error {
		return nil
	})

	require.NoError(t, err)
	assert.False(t, errors.Is(err, &retry.Err{}))
}

func TestUntilAttempts(t *testing.T) {
	ctx := context.Background()
	err := retry.Until(ctx, retry.Attempts(10, time.Millisecond*10), func(ctx context.Context, att int) error {
		return fmt.Errorf("failed attempt '%d'", att)
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, &retry.Err{}))
	assert.Equal(t, "on attempt '10'; attempts exhausted: failed attempt '10'", err.Error())
}

func TestUntilStopped(t *testing.T) {
	ctx := context.Background()
	err := retry.Until(ctx, retry.Attempts(10, time.Millisecond*10), func(ctx context.Context, att int) error {
		return retry.Stop(fmt.Errorf("failed attempt '%d'", att))
	})
	require.Error(t, err)
	// Inspect the error
	var retryErr *retry.Err
	assert.True(t, errors.As(err, &retryErr))
	assert.Equal(t, 1, retryErr.Attempts)
	assert.Equal(t, retry.Stopped, retryErr.Reason)
	assert.Equal(t, "on attempt '1'; retry stopped: failed attempt '1'", err.Error())
}

func TestUntilExponential(t *testing.T) {
	ctx := context.Background()
	backOff := &retry.ExponentialBackOff{
		Min:      time.Millisecond,
		Max:      time.Millisecond * 100,
		Factor:   2,
		Attempts: 10,
	}

	err := retry.Until(ctx, backOff, func(ctx context.Context, att int) error {
		return fmt.Errorf("failed attempt '%d'", att)
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, &retry.Err{}))
	assert.Equal(t, "on attempt '11'; attempts exhausted: failed attempt '11'", err.Error())
}

func TestUntilExponentialCancelled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	backOff := &retry.ExponentialBackOff{
		Min:    time.Millisecond,
		Max:    time.Millisecond * 100,
		Factor: 2,
	}

	err := retry.Until(ctx, backOff, func(ctx context.Context, att int) error {
		return fmt.Errorf("failed attempt '%d'", att)
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, &retry.Err{}))
	assert.Equal(t, "on attempt '6'; context cancelled: failed attempt '6'", err.Error())
}

func TestAsync(t *testing.T) {
	ctx := context.Background()
	async := retry.NewRetryAsync()
	a1 := async.Async("one", ctx, retry.Attempts(10, time.Millisecond*10), func(ctx context.Context, i int) error { return errCause })
	a2 := async.Async("two", ctx, retry.Attempts(10, time.Millisecond*10), func(ctx context.Context, i int) error { return errCause })
	a3 := async.Async("thr", ctx, retry.Attempts(10, time.Millisecond*10), func(ctx context.Context, i int) error { return errCause })

	// Creates the async retry
	f1 := async.Async("for", ctx, retry.Attempts(10, time.Millisecond*100), func(ctx context.Context, i int) error { return errCause })
	// Returns a handler to the currently running async retry
	f2 := async.Async("for", ctx, retry.Attempts(10, time.Millisecond*100), func(ctx context.Context, i int) error { return errCause })

	// The are the same
	assert.Equal(t, f1, f2)
	// Should contain the error for our inspection
	assert.Equal(t, errCause, f2.Err)
	// Should report that the retry is still running
	assert.Equal(t, true, f2.Retrying)

	// Retries are all still running
	time.Sleep(time.Millisecond * 10)
	assert.Equal(t, 4, async.Len())

	// We can inspect the errors for all running async retries
	errs := async.Errs()
	require.NotNil(t, errs)
	for _, e := range errs {
		assert.Equal(t, e, errCause)
	}

	// Wait for all the async retries to exhaust their timeouts
	async.Wait()

	require.Equal(t, errCause, a1.Err)
	require.Equal(t, errCause, a2.Err)
	require.Equal(t, errCause, a3.Err)
	require.Equal(t, errCause, f1.Err)
	require.Equal(t, errCause, f2.Err)
}

func TestBackoffRace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	backOff := &retry.ExponentialBackOff{
		Min:    time.Millisecond,
		Max:    time.Millisecond * 100,
		Factor: 2,
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = retry.Until(ctx, backOff, func(ctx context.Context, att int) error {
				t.Logf("Attempts: %d", backOff.NumRetries())
				return fmt.Errorf("failed attempt '%d'", att)
			})
		}()
	}
	wg.Wait()
}

func TestBackOffNew(t *testing.T) {
	backOff := &retry.ExponentialBackOff{
		Min:    time.Millisecond,
		Max:    time.Millisecond * 100,
		Factor: 2,
	}
	bo := backOff.New()
	assert.Equal(t, bo, backOff)
}
