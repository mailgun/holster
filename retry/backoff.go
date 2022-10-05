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
	NextIteration() time.Duration
	CalcDuration(int64) time.Duration
	Reset()
}

func Interval(t time.Duration) *ConstBackOff {
	return &ConstBackOff{Interval: t}
}

// ConstBackOff indefinitely retry, returns `interval` for each call to Next()
type ConstBackOff struct {
	Interval time.Duration
	retries  int64
}

func (b *ConstBackOff) NumRetries() int                          { return int(atomic.LoadInt64(&b.retries)) }
func (b *ConstBackOff) CalcDuration(retries int64) time.Duration { return b.Interval }
func (b *ConstBackOff) Reset()                                   {}
func (b *ConstBackOff) Next() (time.Duration, bool) {
	atomic.AddInt64(&b.retries, 1)
	return b.Interval, true
}

func (b *ConstBackOff) NextIteration() time.Duration {
	atomic.AddInt64(&b.retries, 1)
	return b.Interval
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

// AttemptsBackOff retry for `attempts` number of retries sleeping for `interval` between each retry
type AttemptsBackOff struct {
	Interval time.Duration
	Attempts int64
	retries  int64
}

func (b *AttemptsBackOff) NumRetries() int                          { return int(atomic.LoadInt64(&b.retries)) }
func (b *AttemptsBackOff) CalcDuration(retries int64) time.Duration { return b.Interval }
func (b *AttemptsBackOff) Reset()                                   { atomic.StoreInt64(&b.retries, 0) }
func (b *AttemptsBackOff) Next() (time.Duration, bool) {
	retries := atomic.AddInt64(&b.retries, 1)
	if retries < b.Attempts {
		return b.Interval, true
	}
	return b.Interval, false
}

func (b *AttemptsBackOff) NextIteration() time.Duration {
	atomic.AddInt64(&b.retries, 1)
	return b.Interval
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
	interval := b.CalcDuration(retries)
	if b.Attempts != 0 && retries > b.Attempts {
		return interval, false
	}
	return interval, true
}

// NextIteration returns the next backoff duration. Is identical to Next() but
// never indicates if we have reached our max attempts
func (b *ExponentialBackOff) NextIteration() time.Duration {
	d, _ := b.Next()
	return d
}

// New returns a copy of the current backoff
func (b *ExponentialBackOff) New() BackOff {
	return &ExponentialBackOff{
		retries:  atomic.LoadInt64(&b.retries),
		Attempts: b.Attempts,
		Factor:   b.Factor,
		Min:      b.Min,
		Max:      b.Max,
	}
}

// CalcDuration returns the next duration to sleep given the number of retries.
//
// This function is useful when keeping track of the attempts and attempt
// resets is external to our code.
func (b *ExponentialBackOff) CalcDuration(retries int64) time.Duration {
	// TODO(thrawn01): Implement jitter?

	d := time.Duration(float64(b.Min) * math.Pow(b.Factor, float64(retries)))
	if d > b.Max {
		return b.Max
	}
	if d < b.Min {
		return b.Min
	}
	return d
}
