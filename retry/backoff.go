package retry

import (
	"math"
	"sync/atomic"
	"time"
)

type BackOff interface {
	New() BackOff
	Next() (time.Duration, bool)
	NumRetries() int
	Reset()
}

func Interval(t time.Duration) *ConstBackOff {
	return &ConstBackOff{Interval: t}
}

// Retry indefinitely sleeping for `interval` between each retry
type ConstBackOff struct {
	Interval time.Duration
	retries  int64
}

func (b *ConstBackOff) NumRetries() int { return int(atomic.LoadInt64(&b.retries)) }
func (b *ConstBackOff) Reset()          {}
func (b *ConstBackOff) Next() (time.Duration, bool) {
	atomic.AddInt64(&b.retries, 1)
	return b.Interval, true
}
func (b *ConstBackOff) New() BackOff {
	return &ConstBackOff{
		retries:  atomic.LoadInt64(&b.retries),
		Interval: b.Interval,
	}
}

func Attempts(a int, t time.Duration) *AttemptsBackOff {
	return &AttemptsBackOff{Interval: t, Attempts: int64(a)}
}

// Retry for `attempts` number of retries sleeping for `interval` between each retry
type AttemptsBackOff struct {
	Interval time.Duration
	Attempts int64
	retries  int64
}

func (b *AttemptsBackOff) NumRetries() int { return int(atomic.LoadInt64(&b.retries)) }
func (b *AttemptsBackOff) Reset()          { atomic.StoreInt64(&b.retries, 0) }
func (b *AttemptsBackOff) Next() (time.Duration, bool) {
	retries := atomic.AddInt64(&b.retries, 1)
	if retries < b.Attempts {
		return b.Interval, true
	}
	return b.Interval, false
}
func (b *AttemptsBackOff) New() BackOff {
	return &AttemptsBackOff{
		retries:  atomic.LoadInt64(&b.retries),
		Interval: b.Interval,
		Attempts: b.Attempts,
	}
}

type ExponentialBackOff struct {
	Min, Max time.Duration
	Factor   float64
	Attempts int64
	retries  int64
}

func (b *ExponentialBackOff) NumRetries() int { return int(atomic.LoadInt64(&b.retries)) }
func (b *ExponentialBackOff) Reset()          { atomic.StoreInt64(&b.retries, 0) }
func (b *ExponentialBackOff) Next() (time.Duration, bool) {
	retries := atomic.AddInt64(&b.retries, 1)
	interval := b.nextInterval(retries)
	if b.Attempts != 0 && retries > b.Attempts {
		return interval, false
	}
	return interval, true
}
func (b *ExponentialBackOff) New() BackOff {
	return &ExponentialBackOff{
		retries:  atomic.LoadInt64(&b.retries),
		Attempts: b.Attempts,
		Factor:   b.Factor,
		Min:      b.Min,
		Max:      b.Max,
	}
}

func (b *ExponentialBackOff) nextInterval(retries int64) time.Duration {
	d := time.Duration(float64(b.Min) * math.Pow(b.Factor, float64(retries)))
	if d > b.Max {
		return b.Max
	}
	if d < b.Min {
		return b.Min
	}
	return d
}
