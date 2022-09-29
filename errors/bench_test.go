//go:build go1.7
// +build go1.7

package errors

import (
	"fmt"
	"testing"

	stderrors "errors"
)

func noErrors(at, depth int) error {
	if at >= depth {
		return stderrors.New("no error")
	}
	return noErrors(at+1, depth)
}

func yesErrors(at, depth int) error {
	if at >= depth {
		return New("ye error")
	}
	return yesErrors(at+1, depth)
}

func BenchmarkErrors(b *testing.B) {
	type run struct {
		stack int
		std   bool
	}
	runs := []run{
		{10, false},
		{10, true},
		{100, false},
		{100, true},
		{1000, false},
		{1000, true},
	}
	for _, r := range runs {
		part := "pkg/errors"
		if r.std {
			part = "errors"
		}
		name := fmt.Sprintf("%s-stack-%d", part, r.stack)
		b.Run(name, func(b *testing.B) {
			f := yesErrors
			if r.std {
				f = noErrors
			}
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = f(0, r.stack)
			}
			b.StopTimer()
		})
	}
}
