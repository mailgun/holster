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
	"os"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/mailgun/holster/v4/errors"
	"github.com/sirupsen/logrus"
)

// Functional test context.
type T struct {
	name     string
	ctx      context.Context
	deadline time.Time
	pass     bool
	indent   int
}

// Functional test code.
type TestFunc func(t *T)

var (
	log        = logrus.WithField("category", "functional")
	maxTimeout = 10 * time.Minute
)

func newT(name string) *T {
	return &T{
		name: name,
		pass: true,
	}
}

func (t *T) Name() string {
	return t.name
}

func (t *T) Run(name string, fn TestFunc) bool {
	t2 := &T{
		name:   joinName(t.name, name),
		indent: t.indent + 1,
		pass:   true,
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

func (t *T) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	t.pass = false
}

func (t *T) FailNow() {
	panic("")
}

func (t *T) Log(message ...interface{}) {
	fmt.Println(message...)
}

func (t *T) Logf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (t *T) invoke(ctx context.Context, fn TestFunc) {
	startTime := time.Now()

	if ctx.Err() != nil {
		panic(ctx.Err())
	}

	t.deadline = time.Now().Add(maxTimeout)
	ctx, cancel := context.WithDeadline(ctx, t.deadline)
	defer cancel()
	t.ctx = ctx
	t.Logf("≈≈≈ RUN   %s", t.name)

	func() {
		defer func() {
			// Handle panic.
			if err := recover(); err != nil {
				errMsg := fmt.Sprintf("%v", err)
				if errMsg != "" {
					log.WithField("test", t.name).Error(errMsg)
				}
				// TODO: Print stack trace.

				t.pass = false
			}
		}()

		fn(t)
	}()

	elapsed := time.Now().Sub(startTime)
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
