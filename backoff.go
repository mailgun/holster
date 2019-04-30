package holster

import (
	"math"
	"time"
)

var backoff = NewBackOff(time.Millisecond*300, time.Second*30, 2)

type BackOffCounter struct {
	min, max time.Duration
	factor   float64
	attempt  int
}

func NewBackOff(min, max time.Duration, factor float64) *BackOffCounter {
	return &BackOffCounter{
		factor: factor,
		min:    min,
		max:    max,
	}
}

// Next returns the next back off duration based on the number of
// times Next() was called. Each call to next returns the next factor
// of back off. Call Reset() to reset the back off attempts to zero.
func (b *BackOffCounter) Next() time.Duration {
	d := b.BackOff(b.attempt)
	b.attempt++
	return d
}

// Reset sets the back off attempt counter to zero
func (b *BackOffCounter) Reset() {
	b.attempt = 0
}

// BackOff calculates the back depending on the attempts provided
func (b *BackOffCounter) BackOff(attempt int) time.Duration {
	d := time.Duration(float64(b.min) * math.Pow(b.factor, float64(attempt)))
	if d > b.max {
		return b.max
	}
	if d < b.min {
		return b.min
	}
	return d
}

// BackOff is a convenience function which returns a back off duration
// with a default 300 millisecond minimum and a 30 second maximum with
// a factor of 2 for each attempt.
func BackOff(attempt int) time.Duration {
	return backoff.BackOff(attempt)
}
