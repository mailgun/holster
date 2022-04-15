package steve_test

import (
	"context"
	"io"

	"github.com/mailgun/holster/v4/steve"
	"github.com/stretchr/testify/mock"
)

type MockJob struct {
	mock.Mock
}

func (m *MockJob) Start(ctx context.Context, writer io.Writer, closer *steve.JobCloser) error {
	args := m.Called(ctx, writer, closer)
	return args.Error(0)
}

func (m *MockJob) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
