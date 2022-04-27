package steve

import (
	"context"
	"io"
	"runtime/debug"

	"github.com/mailgun/holster/v4/errors"
)

// Action to perform by an `EZJob`.
type Action interface {
	Run(context.Context, io.Writer) error
	Status(Status)
}

// EZJob implements `steve.Job` to run an `Action` in a goroutine.
// `EZJob` performs error and return value handling.
type EZJob struct {
	Job
	action   Action
	stopChan chan struct{}

	// Context for running job.
	ctx    context.Context
	cancel context.CancelFunc
}

var ErrPanic = errors.New("panic")

func NewEZJob(action Action) *EZJob {
	return &EZJob{
		action: action,
	}
}

func (a *EZJob) Start(ctx context.Context, writer io.Writer, closer *TaskCloser) error {
	a.ctx, a.cancel = context.WithCancel(context.Background())
	a.stopChan = make(chan struct{})

	go func() {
		var reterr error

		defer func() {
			// Handle panic.
			if err := recover(); err != nil {
				stackDump := string(debug.Stack())
				log.Errorf("panic: %v\n%s", err, stackDump)
				reterr = errors.WithStack(ErrPanic)
			}

			// Clean up.
			a.cancel()
			closer.Close(reterr)
			close(a.stopChan)
		}()

		reterr = a.action.Run(a.ctx, writer)
	}()

	return nil
}

func (a *EZJob) Stop(ctx context.Context) error {
	a.cancel()

	select {
	case <-a.stopChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *EZJob) Status(status Status) {
	a.action.Status(status)
}
