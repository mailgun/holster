//go:build go1.13
// +build go1.13

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
			name:   "holster_double_wrap",
			target: target,
			err:    Wrap(Wrap(target, "reason2"), "reason1"),
		},
		{
			name:   "std_wrap",
			target: target,
			err:    fmt.Errorf("some reason: %w", target),
		},
		{
			name:   "std_double_wrap",
			target: target,
			err:    fmt.Errorf("reason1: %w", fmt.Errorf("reason2: %w", target)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, Is(tt.err, tt.target))
			assert.ErrorIs(t, tt.err, tt.target)
		})
	}
}
