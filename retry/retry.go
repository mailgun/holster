package retry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mailgun/holster/v5/errors"
	"github.com/mailgun/holster/v5/syncutil"
)

const (
	Cancelled         = cancelReason("context cancelled")
	Stopped           = cancelReason("retry stopped")
	AttemptsExhausted = cancelReason("attempts exhausted")
)

type Func func(context.Context, int) error

type cancelReason string

type stopErr struct {
	err error
}

func (e *stopErr) Error() string {
	return fmt.Sprintf("stop err: %s", e.err.Error())
}

type Err struct {
	Err      error
	Reason   cancelReason
	Attempts int
}

func (e *Err) Unwrap() error { return e.Err }
func (e *Err) Error() string {
	return fmt.Sprintf("on attempt '%d'; %s: %s", e.Attempts, e.Reason, e.Err.Error())
}

func (e *Err) Is(target error) bool {
	_, ok := target.(*Err)
	return ok
}

// Stop forces the retry to cancel with the provided error
// and retry.Err.Reason == retry.Stopped
func Stop(err error) error {
	return &stopErr{err: err}
}

// Until will retry the provided `retry.Func` until it returns nil or
// the context is cancelled. Optionally users may use `retry.Stop()` to force
// the retry to terminate with an error. Returns a `retry.Err` with
// the included Reason and Attempts
func Until(ctx context.Context, backOff BackOff, f Func) error {
	var attempt int
	for {
		attempt++
		if err := f(ctx, attempt); err != nil {
			var stop *stopErr
			if errors.As(err, &stop) {
				return &Err{Attempts: attempt, Reason: Stopped, Err: stop.err}
			}
			interval, retry := backOff.Next()
			if !retry {
				return &Err{Attempts: attempt, Reason: AttemptsExhausted, Err: err}
			}
			timer := time.NewTimer(interval)
			select {
			case <-timer.C:
				timer.Stop()
				continue
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return &Err{Attempts: attempt, Reason: Cancelled, Err: err}
			}
		}
		return nil
	}
}

type AsyncItem struct {
	Retrying bool
	Attempts int
	Err      error
}

func (s *AsyncItem) Error() string {
	return s.Err.Error()
}

type Async struct {
	asyncs map[interface{}]AsyncItem
	mutex  *sync.Mutex
	ctx    context.Context
	wg     syncutil.WaitGroup
}

// Given a function that takes a context, run the provided function; if it fails, retry the function asynchronously
// and return Async{}. Subsequent calls to with the same 'key' will return Async{} if the function is still
// retrying, this continues until the retry period has exhausted or the context expires or is cancelled.
// Then the final error returned by f() is returned to the caller on the final call with the same 'key'
//
// The code assumes the caller will continue to call `Async()` until either the retries have exhausted or
// an Async{Retrying: false} is returned.
func NewRetryAsync() *Async {
	return &Async{
		mutex:  &sync.Mutex{},
		asyncs: make(map[interface{}]AsyncItem),
	}
}

// Return the number of active async retries
func (s *Async) Len() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return len(s.asyncs)
}

// Stop forces stop of all running async retries
func (s *Async) Stop() {
	s.wg.Stop()
}

// Wait waits for all running async retries to complete
func (s *Async) Wait() {
	s.wg.Wait()
}

func (s *Async) Async(key interface{}, ctx context.Context, bo BackOff,
	f func(context.Context, int) error) *AsyncItem {

	// does this key have an existing retry running?
	s.mutex.Lock()
	if async, ok := s.asyncs[key]; ok {
		// Remove entries that are no longer re-trying
		if !async.Retrying {
			delete(s.asyncs, key)
		}
		s.mutex.Unlock()
		return &async
	}
	s.mutex.Unlock()

	// Attempt to run the function, if successful return nil
	err := f(s.ctx, 0)
	if err == nil {
		return nil
	}

	async := AsyncItem{
		Retrying: true,
		Err:      err,
	}

	s.mutex.Lock()
	s.asyncs[key] = async
	s.mutex.Unlock()

	// Create an go routine to run the retry
	s.wg.Until(func(done chan struct{}) bool {
		async := AsyncItem{Retrying: true}

		for {
			// Retry the function
			async.Attempts++
			async.Err = f(ctx, async.Attempts)

			// If success, then indicate we are no longer retrying
			if async.Err == nil {
				async.Retrying = false

				s.mutex.Lock()
				s.asyncs[key] = async
				s.mutex.Unlock()
				return false
			}

			// Record the error and attempts
			s.mutex.Lock()
			s.asyncs[key] = async
			s.mutex.Unlock()

			interval, retry := bo.Next()
			if !retry {
				async.Retrying = false
				s.mutex.Lock()
				s.asyncs[key] = async
				s.mutex.Unlock()
				return false
			}

			timer := time.NewTimer(interval)
			select {
			case <-timer.C:
				timer.Stop()
			case <-ctx.Done():
				async.Retrying = false

				s.mutex.Lock()
				s.asyncs[key] = async
				s.mutex.Unlock()
				timer.Stop()
				return false
			case <-done:
				// immediate abort, abandon all work
				if !timer.Stop() {
					<-timer.C
				}
				return false
			}
		}
	})
	return &async
}

// Return errors from failed asyncs and clean up the internal async map
func (s *Async) Errs() map[interface{}]AsyncItem {
	results := make(map[interface{}]AsyncItem)
	s.mutex.Lock()

	for key, async := range s.asyncs {
		// Remove entries that are no longer re-trying
		if !async.Retrying {
			delete(s.asyncs, key)

			// Only include async's that had an error
			if async.Err != nil {
				results[key] = async
			}
		}
	}
	s.mutex.Unlock()
	return results
}
