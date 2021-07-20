package cancel

import (
	"context"
	"sync"
	"time"
)

type Context interface {
	context.Context
	Wrap(context.Context) context.Context
	Cancel()
}

type cancelCtx struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a context that wraps the given context and returns an obj that can be cancelled.
// This allows an object which desires to cancel a long running operation to store a single
// cancel.Context in it's struct variables instead of having to store both the context.Context
// and context.CancelFunc.
func New(ctx context.Context) Context {
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithCancel(ctx)
	return &cancelCtx{
		cancel: cancel,
		ctx:    ctx,
	}
}

func (c *cancelCtx) Cancel()                                 { c.cancel() }
func (c *cancelCtx) Deadline() (deadline time.Time, ok bool) { return c.ctx.Deadline() }
func (c *cancelCtx) Done() <-chan struct{}                   { return c.ctx.Done() }
func (c *cancelCtx) Err() error                              { return c.ctx.Err() }
func (c *cancelCtx) Value(key interface{}) interface{}       { return c.ctx.Value(key) }

// Wrap returns a Context that will be cancelled when either cancel.Context or the passed context is cancelled
func (c *cancelCtx) Wrap(ctx context.Context) context.Context {
	return NewWrappedContext(ctx, c)
}

// NewWrappedContext returns a Context that will be cancelled when either of the passed contexts are cancelled
func NewWrappedContext(left, right context.Context) context.Context {
	w := WrappedContext{
		doneCh: make(chan struct{}),
	}
	// Wait for either ctx to be cancelled and propagate to the wrapped context
	go func() {
		select {
		case <-left.Done():
			w.Reason(left.Err())
		case <-right.Done():
			w.Reason(right.Err())
		}
	}()
	return &w
}

type WrappedContext struct {
	context.Context

	mutex  sync.Mutex
	doneCh chan struct{}
	err    error
}

func (w *WrappedContext) Reason(err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	close(w.doneCh)
	w.err = err
}

func (w *WrappedContext) Done() <-chan struct{} {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.doneCh
}

func (w *WrappedContext) Err() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.err
}
