package holster_test

import (
	"testing"
	"time"

	"github.com/mailgun/holster"
	"github.com/stretchr/testify/assert"
)

func TestBackoffFunc(t *testing.T) {
	d := holster.BackOff(0)
	assert.Equal(t, time.Millisecond*300, d)

	d = holster.BackOff(1)
	assert.Equal(t, time.Millisecond*600, d)
}
