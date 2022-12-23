/*
Copyright 2022 Mailgun Technologies Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package functional

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/mailgun/holster/v4/errors"
)

// Functional test context.
type T struct {
	name      string
	ctx       context.Context
	deadline  time.Time
	indent    int
	writer    io.Writer
	errWriter io.Writer
	args      []string
	result    TestResult
}

type TestResult struct {
	Pass      bool
	Skipped   bool
	StartTime time.Time
	EndTime   time.Time
}

// Functional test code.
type TestFunc func(t *T)

var maxTimeout = 10 * time.Minute

func newT(name string, opts ...FunctionalOption) *T {
	t := &T{
		name:      name,
		writer:    os.Stdout,
		errWriter: os.Stderr,
	}

	for _, opt := range opts {
		opt.Apply(t)
	}

	return t
}

func (t *T) Name() string {
	return t.name
}

func (t *T) Run(name string, fn TestFunc) bool {
	t2 := &T{
		name:      joinName(t.name, name),
		indent:    t.indent + 1,
		writer:    t.writer,
		errWriter: t.errWriter,
	}

	t2.invoke(t.ctx, fn)

	if !t2.result.Pass {
		t.result.Pass = false
	}

	return t.result.Pass
}

func (t *T) Deadline() (time.Time, error) {
	if t.deadline.IsZero() {
		return time.Time{}, errors.New("Deadline not set")
	}
	return t.deadline, nil
}

func (t *T) Error(args ...any) {
	fmt.Fprintln(t.errWriter, args...)
	t.result.Pass = false
}

func (t *T) Errorf(format string, args ...any) {
	fmt.Fprintf(t.errWriter, format+"\n", args...)
	t.result.Pass = false
}

func (t *T) FailNow() {
	panic("")
}

func (t *T) Log(message ...any) {
	if len(message) > 0 {
		fmt.Fprintln(t.writer, message...)
	}
}

func (t *T) Logf(format string, args ...any) {
	fmt.Fprintf(t.writer, format+"\n", args...)
}

func (t *T) Args() []string {
	return t.args
}

func (t *T) Skip(args ...any) {
	t.Log(args...)
	t.SkipNow()
}

func (t *T) Skipf(format string, args ...any) {
	t.Logf(format, args...)
	t.SkipNow()
}

func (t *T) Skipped() bool {
	return t.result.Skipped
}

func (t *T) SkipNow() {
	t.result.Skipped = true
	runtime.Goexit()
}

func (t *T) invoke(ctx context.Context, fn TestFunc) {
	callFn := func() {
		fn(t)
	}
	t.commonInvoke(ctx, callFn, nil)
}

func (t *T) commonInvoke(ctx context.Context, fn, postHandler func()) {
	if ctx.Err() != nil {
		panic(ctx.Err())
	}

	t.deadline = time.Now().Add(maxTimeout)
	ctx, cancel := context.WithDeadline(ctx, t.deadline)
	defer cancel()
	t.ctx = ctx
	t.result.Pass = true
	t.Logf("≈≈≈ RUN   %s", t.name)
	t.result.StartTime = time.Now()

	// Call test in goroutine.
	done := make(chan any)
	go func() {
		var finished bool
		defer func() {
			t.result.Skipped = !finished
			done <- recover()
		}()

		fn()
		finished = true
	}()

	// Wait, then handle panic.
	if fnErr := <-done; fnErr != nil {
		errMsg := fmt.Sprintf("%v", fnErr)
		if errMsg != "" {
			t.Error(errMsg)
		}
		t.Error(debug.Stack())
	}

	t.result.EndTime = time.Now()
	elapsed := t.result.EndTime.Sub(t.result.StartTime)

	if postHandler != nil {
		postHandler()
	}

	switch {
	case t.result.Skipped:
		t.Logf("⁓⁓⁓ SKIP: %s (%s)", t.name, elapsed)
	case t.result.Pass:
		t.Logf("⁓⁓⁓ PASS: %s (%s)", t.name, elapsed)
	default:
		t.Logf("⁓⁓⁓ FAIL: %s (%s)", t.name, elapsed)
	}
}

// Get base name of function.
func funcName(fn any) string {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return name
	}
	return name[idx+1:]
}

func joinName(a, b string) string {
	return a + "/" + b
}
