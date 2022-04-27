package steve_test

import (
	"context"
	"io"

	"github.com/mailgun/holster/v4/steve"
)

// Stub `steve.Job` implementation.
type FakeJob struct {
	JobId      steve.JobId
	LastStatus steve.Status
}

func (a *FakeJob) Start(_ context.Context, _ io.Writer, _ *steve.TaskCloser) error {
	return nil
}

func (a *FakeJob) Stop(_ context.Context) error {
	return nil
}

func (a *FakeJob) Status(status steve.Status) {
	a.LastStatus = status
}
