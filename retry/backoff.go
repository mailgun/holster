package retry

import (
	"math"
	"time"
)

type Backoff interface {
	Next() (time.Duration, bool)
	Reset()
}

func Interval(t time.Duration) *ConstBackoff {
	return &ConstBackoff{Interval: t}
}

// Retry indefinitely sleeping for `interval` between each retry
type ConstBackoff struct {
	Interval time.Duration
	retries  int
}

func (b *ConstBackoff) Reset() {}
func (b *ConstBackoff) Next() (time.Duration, bool) {
	b.retries++
	return b.Interval, true
}

func Attempts(a int, t time.Duration) *AttemptsBackoff {
	return &AttemptsBackoff{Interval: t, Attempts: a}
}

// Retry for `attempts` number of retries sleeping for `interval` between each retry
type AttemptsBackoff struct {
	Interval time.Duration
	Attempts int
	retries  int
}

func (b *AttemptsBackoff) Reset() { b.retries = 0 }
func (b *AttemptsBackoff) Next() (time.Duration, bool) {
	b.retries++
	if b.retries < b.Attempts {
		return b.Interval, true
	}
	return b.Interval, false
}

type ExponentialBackoff struct {
	Min, Max time.Duration
	Factor   float64
	Attempts int
	retries  int
}

func (b *ExponentialBackoff) Reset() { b.retries = 0 }
func (b *ExponentialBackoff) Next() (time.Duration, bool) {
	interval := b.nextInterval()
	b.retries++
	if b.Attempts != 0 && b.retries > b.Attempts {
		return interval, false
	}
	return interval, true
}

func (b *ExponentialBackoff) nextInterval() time.Duration {
	d := time.Duration(float64(b.Min) * math.Pow(b.Factor, float64(b.retries)))
	if d > b.Max {
		return b.Max
	}
	if d < b.Min {
		return b.Min
	}
	return d
}
