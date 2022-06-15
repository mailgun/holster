package functional

import (
	"context"
	"fmt"
	"time"
)

// Functional benchmark context.
type B struct {
	T
	N int

	// Mean nanoseconds per operation.
	nsPerOp float64
}

// Functional test code.
type BenchmarkFunc func(b *B)

type BenchmarkResult struct {
	Pass bool
	// Mean nanoseconds per operation.
	NsPerOp float64
}

func newB(name string, times int) *B {
	return &B{
		T: T{
			name: name,
		},
		N: times,
	}
}

func (b *B) Run(name string, fn BenchmarkFunc) BenchmarkResult {
	return b.RunTimes(name, fn, 1)
}

func (b *B) RunTimes(name string, fn BenchmarkFunc, times int) BenchmarkResult {
	b2 := &B{
		T: T{
			name:   joinName(b.name, name),
			indent: b.indent + 1,
		},
		N: times,
	}

	b2.invoke(b.T.ctx, fn)

	if !b2.pass {
		b.pass = false
	}

	return b.result()
}

func (b *B) invoke(ctx context.Context, fn BenchmarkFunc) {
	if ctx.Err() != nil {
		panic(ctx.Err())
	}

	b.deadline = time.Now().Add(maxTimeout)
	ctx, cancel := context.WithDeadline(ctx, b.deadline)
	defer cancel()
	b.ctx = ctx
	b.pass = true
	b.Logf("≈≈≈ RUN   %s", b.name)
	startTime := time.Now()

	func() {
		defer func() {
			// Handle panic.
			if err := recover(); err != nil {
				errMsg := fmt.Sprintf("%v", err)
				if errMsg != "" {
					log.WithField("test", b.name).Error(errMsg)
				}
				// TODO: Print stack trace.

				b.pass = false
			}
		}()

		fn(b)
	}()

	endTime := time.Now()
	elapsed := endTime.Sub(startTime)
	b.nsPerOp = float64(elapsed.Nanoseconds()) / float64(b.N)
	nsPerOpDur := time.Duration(int64(b.nsPerOp))
	b.Logf("%s\t%d\t%s ns/op (%s/op)", b.name, b.N, formatFloat(b.nsPerOp), nsPerOpDur.String())
	if b.pass {
		b.Logf("⁓⁓⁓ PASS: %s (%s)", b.name, elapsed)
	} else {
		b.Logf("⁓⁓⁓ FAIL: %s (%s)", b.name, elapsed)
	}
}

func (b *B) result() BenchmarkResult {
	return BenchmarkResult{
		Pass:    b.pass,
		NsPerOp: b.nsPerOp,
	}
}

// Format float as human readable string with up to 5 decimal places.
func formatFloat(d float64) string {
	str := fmt.Sprintf("%0.5f", d)

	// Strip insignificant zeros from right.
	var i int
	for i = len(str) - 1; i > 0; i-- {
		if str[i] == '.' {
			return str[0:i]
		}
		if str[i] != '0' {
			return str[0 : i+1]
		}
	}

	return str
}
