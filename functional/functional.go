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

// `go test`-like functional testing framework.
// Can be used with Testify require/assert/mock.
package functional

import (
	"context"
	"time"
)

// Run a test.  Test named after function name.
func Run(ctx context.Context, fn TestFunc) bool {
	name := funcName(fn)
	t := newT(name)
	t.invoke(ctx, fn)
	return t.pass
}

// Run a test with user-provided name.
func RunWithName(ctx context.Context, name string, fn TestFunc) bool {
	t := newT(name)
	t.invoke(ctx, fn)
	return t.pass
}

// Run a suite of tests as a unit.
// Generates summary when finished.
func RunSuite(ctx context.Context, suiteName string, tests []TestFunc) bool {
	suiteStartTime := time.Now()
	result := map[bool]int{true: 0, false: 0}
	numTests := len(tests)
	t := &T{
		name: suiteName,
		pass: true,
	}

	t.invoke(ctx, func(t *T) {
		for _, test := range tests {
			testName := funcName(test)
			pass := t.Run(testName, test)
			result[pass]++
		}

		pass := result[false] == 0
		passPct := float64(result[true]) / float64(numTests) * 100
		t.Log()
		t.Log("Suite test result summary:")
		t.Logf("    pass: %d (%0.1f%%)", result[true], passPct)
		t.Logf("    fail: %d", result[false])
		t.Logf("    elapsed: %s", time.Now().Sub(suiteStartTime))

		if !pass {
			t.FailNow()
		}
	})

	return t.pass
}
