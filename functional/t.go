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
	"github.com/sirupsen/logrus"
)

// Functional test context.
type T struct {
	name      string
	ctx       context.Context
	deadline  time.Time
	pass      bool
	indent    int
	writer    io.Writer
	errWriter io.Writer
	args      []string
}

// Functional test code.
type TestFunc func(t *T)

var (
	log        = logrus.WithField("category", "functional")
	maxTimeout = 10 * time.Minute
)

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

	if !t2.pass {
		t.pass = false
	}

	return t.pass
}

func (t *T) Deadline() (time.Time, error) {
	if t.deadline.IsZero() {
		return time.Time{}, errors.New("Deadline not set")
	}
	return t.deadline, nil
}

func (t *T) Error(args ...interface{}) {
	fmt.Fprintln(t.errWriter, args...)
	t.pass = false
}

func (t *T) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(t.errWriter, format+"\n", args...)
	t.pass = false
}

func (t *T) FailNow() {
	panic("")
}

func (t *T) Log(message ...interface{}) {
	fmt.Fprintln(t.writer, message...)
}

func (t *T) Logf(format string, args ...interface{}) {
	fmt.Fprintf(t.writer, format+"\n", args...)
}

func (t *T) Args() []string {
	return t.args
}

func (t *T) invoke(ctx context.Context, fn TestFunc) {
	if ctx.Err() != nil {
		panic(ctx.Err())
	}

	t.deadline = time.Now().Add(maxTimeout)
	ctx, cancel := context.WithDeadline(ctx, t.deadline)
	defer cancel()
	t.ctx = ctx
	t.pass = true
	t.Logf("≈≈≈ RUN   %s", t.name)
	startTime := time.Now()

	func() {
		defer func() {
			// Handle panic.
			if err := recover(); err != nil {
				errMsg := fmt.Sprintf("%v", err)
				if errMsg != "" {
					t.Error(errMsg)
				}
				t.Error(debug.Stack())
			}
		}()

		fn(t)
	}()

	endTime := time.Now()
	elapsed := endTime.Sub(startTime)
	if t.pass {
		t.Logf("⁓⁓⁓ PASS: %s (%s)", t.name, elapsed)
	} else {
		t.Logf("⁓⁓⁓ FAIL: %s (%s)", t.name, elapsed)
	}
}

// Get base name of function.
func funcName(fn interface{}) string {
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
