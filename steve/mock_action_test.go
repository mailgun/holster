package steve_test

import (
	"context"
	"io"

	"github.com/mailgun/holster/v4/steve"
	"github.com/stretchr/testify/mock"
)

type MockAction struct {
	mock.Mock
}

func (m *MockAction) Run(ctx context.Context, output io.Writer) error {
	args := m.Called(ctx, output)
	return args.Error(0)
}

func (m *MockAction) Status(status steve.Status) {
	m.Called(status)
}
