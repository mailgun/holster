package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIs(t *testing.T) {
	target := New("wrapped")

	err := Wrap(target, "some reason")

	is := Is(err, target)
	assert.True(t, is)
	assert.ErrorIs(t, err, target)
}

func TestStdIs(t *testing.T) {
	target := New("wrapped")

	err := fmt.Errorf("some reason: %w", target)

	is := Is(err, target)
	assert.True(t, is)
	assert.ErrorIs(t, err, target)
}
