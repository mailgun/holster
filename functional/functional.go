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
func Run(ctx context.Context, fn TestFunc, opts ...FunctionalOption) bool {
	name := funcName(fn)
	t := newT(name, opts...)
	t.invoke(ctx, fn)
	return t.result.Pass
}

// Run a test with user-provided name.
func RunWithName(ctx context.Context, name string, fn TestFunc, opts ...FunctionalOption) bool {
	t := newT(name, opts...)
	t.invoke(ctx, fn)
	return t.result.Pass
}

// Run a suite of tests as a unit.
// Generates summary when finished.
func RunSuite(ctx context.Context, suiteName string, tests []TestFunc, opts ...FunctionalOption) bool {
	result := map[bool]int{true: 0, false: 0}
	numTests := len(tests)
	t := newT(suiteName, opts...)
	suiteStartTime := time.Now()

	t.invoke(ctx, func(t *T) {
		for _, test := range tests {
			testName := funcName(test)
			pass := t.Run(testName, test)
			result[pass]++
		}

		suiteEndTime := time.Now()
		pass := result[false] == 0
		passPct := float64(result[true]) / float64(numTests) * 100
		t.Log()
		t.Log("Suite test result summary:")
		t.Logf("    pass: %d (%0.1f%%)", result[true], passPct)
		t.Logf("    fail: %d", result[false])
		t.Logf("    elapsed: %s", suiteEndTime.Sub(suiteStartTime))

		if !pass {
			t.FailNow()
		}
	})

	return t.result.Pass
}

// Run a benchmark test.  Test named after function name.
func RunBenchmarkTimes(ctx context.Context, fn BenchmarkFunc, times int, opts ...FunctionalOption) TestResult {
	name := funcName(fn)
	b := newB(name, times, opts...)
	b.invoke(ctx, fn)
	return b.result
}

// Run a benchmark test with user-provided name.
func RunBenchmarkTimesWithName(ctx context.Context, name string, fn BenchmarkFunc, times int, opts ...FunctionalOption) TestResult {
	b := newB(name, times, opts...)
	b.invoke(ctx, fn)
	return b.result
}

// Run a suite of benchmark tests as a unit.
// Run each benchmark n times.
// Generates summary when finished.
func RunBenchmarkSuiteTimes(ctx context.Context, suiteName string, times int, tests []BenchmarkFunc, opts ...FunctionalOption) bool {
	result := map[bool]int{true: 0, false: 0}
	numTests := len(tests)
	b := newB(suiteName, 1, opts...)
	suiteStartTime := time.Now()

	b.invoke(ctx, func(b *B) {
		for _, test := range tests {
			testName := funcName(test)
			b.N = times
			bret := b.Run(testName, test)
			result[bret.Pass]++
		}

		suiteEndTime := time.Now()
		pass := result[false] == 0
		passPct := float64(result[true]) / float64(numTests) * 100
		b.Log()
		b.Log("Suite benchmark test result summary:")
		b.Logf("    pass: %d (%0.1f%%)", result[true], passPct)
		b.Logf("    fail: %d", result[false])
		b.Logf("    elapsed: %s", suiteEndTime.Sub(suiteStartTime))

		if !pass {
			b.FailNow()
		}
	})

	return b.result.Pass
}
