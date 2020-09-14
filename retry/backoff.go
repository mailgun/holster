package retry

import (
	"math"
	"sync/atomic"
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
	retries  int64
}

func (b *ConstBackoff) Reset() {}
func (b *ConstBackoff) Next() (time.Duration, bool) {
	atomic.AddInt64(&b.retries, 1)
	return b.Interval, true
}

func Attempts(a int, t time.Duration) *AttemptsBackoff {
	return &AttemptsBackoff{Interval: t, Attempts: int64(a)}
}

// Retry for `attempts` number of retries sleeping for `interval` between each retry
type AttemptsBackoff struct {
	Interval time.Duration
	Attempts int64
	retries  int64
}

func (b *AttemptsBackoff) Reset() { atomic.StoreInt64(&b.retries, 0) }
func (b *AttemptsBackoff) Next() (time.Duration, bool) {
	retries := atomic.AddInt64(&b.retries, 1)
	if retries < b.Attempts {
		return b.Interval, true
	}
	return b.Interval, false
}

type ExponentialBackoff struct {
	Min, Max time.Duration
	Factor   float64
	Attempts int64
	retries  int64
}

func (b *ExponentialBackoff) Reset() { atomic.StoreInt64(&b.retries, 0) }
func (b *ExponentialBackoff) Next() (time.Duration, bool) {
	retries := atomic.AddInt64(&b.retries, 1)
	interval := b.nextInterval(retries)
	if b.Attempts != 0 && retries > b.Attempts {
		return interval, false
	}
	return interval, true
}

func (b *ExponentialBackoff) nextInterval(retries int64) time.Duration {
	d := time.Duration(float64(b.Min) * math.Pow(b.Factor, float64(retries)))
	if d > b.Max {
		return b.Max
	}
	if d < b.Min {
		return b.Min
	}
	return d
}
