package functional

import (
	"context"
	"fmt"
	"os"
	"time"
)

// Functional benchmark context.
type B struct {
	T
	N    int
	leaf bool

	// Mean nanoseconds per operation.
	nsPerOp   float64
	startTime time.Time
}

// Functional test code.
type BenchmarkFunc func(b *B)

type BenchmarkResult struct {
	Pass bool
	// Mean nanoseconds per operation.
	NsPerOp float64
}

func newB(name string, times int, opts ...FunctionalOption) *B {
	b := &B{
		T: T{
			name:      name,
			writer:    os.Stdout,
			errWriter: os.Stderr,
		},
		N:    times,
		leaf: true,
	}

	for _, opt := range opts {
		opt.Apply(&b.T)
	}

	return b
}

func (b *B) Run(name string, fn BenchmarkFunc, opts ...FunctionalOption) BenchmarkResult {
	b.leaf = false
	return b.RunTimes(name, fn, b.N, opts...)
}

func (b *B) RunTimes(name string, fn BenchmarkFunc, times int, opts ...FunctionalOption) BenchmarkResult {
	b2 := &B{
		T: T{
			name:      joinName(b.name, name),
			indent:    b.indent + 1,
			writer:    b.writer,
			errWriter: b.errWriter,
		},
		N:    times,
		leaf: true,
	}

	b2.invoke(b.T.ctx, fn)

	if !b2.pass {
		b.pass = false
	}

	return b.result()
}

func (b *B) ResetTimer() {
	b.startTime = time.Now()
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
	b.startTime = time.Now()

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
	elapsed := endTime.Sub(b.startTime)
	b.nsPerOp = float64(elapsed.Nanoseconds()) / float64(b.N)

	if b.leaf {
		nsPerOpDur := time.Duration(int64(b.nsPerOp))
		b.Logf("%s\t%d\t%s ns/op (%s/op)", b.name, b.N, formatFloat(b.nsPerOp), nsPerOpDur.String())
	} else {
		b.Logf("%s", b.name)
	}

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
