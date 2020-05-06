package etcdutil

import (
	"math"
	"time"
)

type backOffCounter struct {
	min, max time.Duration
	factor   float64
	attempt  int
}

func newBackOffCounter(min, max time.Duration, factor float64) *backOffCounter {
	return &backOffCounter{
		factor: factor,
		min:    min,
		max:    max,
	}
}

// Next returns the next back off duration based on the number of
// times Next() was called. Each call to next returns the next factor
// of back off. Call Reset() to reset the back off attempts to zero.
func (b *backOffCounter) Next() time.Duration {
	d := b.BackOff(b.attempt)
	b.attempt++
	return d
}

// Reset sets the back off attempt counter to zero
func (b *backOffCounter) Reset() {
	b.attempt = 0
}

// BackOff calculates the back depending on the attempts provided
func (b *backOffCounter) BackOff(attempt int) time.Duration {
	d := time.Duration(float64(b.min) * math.Pow(b.factor, float64(attempt)))
	if d > b.max {
		return b.max
	}
	if d < b.min {
		return b.min
	}
	return d
}
