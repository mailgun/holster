package functional

import (
	"context"
	"os"
	"time"
)

// Functional benchmark context.
type B struct {
	T
	N    int
	leaf bool
}

// Functional test code.
type BenchmarkFunc func(b *B)

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

func (b *B) Run(name string, fn BenchmarkFunc, opts ...FunctionalOption) TestResult {
	b.leaf = false
	longname := joinName(b.name, name)
	b2 := newB(longname, b.N)
	b2.indent++
	b2.writer = b.writer
	b2.errWriter = b.errWriter

	b2.invoke(b.T.ctx, fn)

	if !b2.result.Pass {
		b.result.Pass = false
	}

	return b.result
}

func (b *B) ResetTimer() {
	b.result.StartTime = time.Now()
}

func (b *B) invoke(ctx context.Context, fn BenchmarkFunc) {
	callFn := func() {
		fn(b)
	}
	b.commonInvoke(ctx, callFn, b.postHandler)
}

func (b *B) postHandler() {
	// Benchmark stats.
	if b.leaf && b.N > 0 {
		elapsed := b.result.EndTime.Sub(b.result.StartTime)
		nsPerOp := float64(elapsed.Nanoseconds()) / float64(b.N)
		perOp := time.Duration(int64(nsPerOp))
		b.Logf("%s\t%d\t%s/op", b.name, b.N, perOp.String())
	}
}
