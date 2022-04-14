package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIs(t *testing.T) {
	target := New("wrapped")

	tests := []struct {
		name   string
		target error
		err    error
	}{
		{
			name:   "holster_wrap",
			target: target,
			err:    Wrap(target, "some reason"),
		},
		{
			name:   "std_wrap",
			target: target,
			err:    fmt.Errorf("some reason: %w", target),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, Is(tt.err, tt.target))
			assert.ErrorIs(t, tt.err, tt.target)
		})
	}
}
