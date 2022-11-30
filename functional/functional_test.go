package functional_test

import (
	"bufio"
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/mailgun/holster/v4/functional"
	"github.com/stretchr/testify/assert"
)

func TestFunctional(t *testing.T) {
	ctx := context.Background()

	t.Run("Run()", func(t *testing.T) {
		t.Run("Happy path", func(t *testing.T) {
			testFunc := func(_ *functional.T) {}
			pass := functional.Run(ctx, testFunc)
			assert.True(t, pass)
		})

		t.Run("Test fails", func(t *testing.T) {
			testFunc := func(ft *functional.T) {
				ft.FailNow()
			}
			pass := functional.Run(ctx, testFunc)
			assert.False(t, pass)
		})

		t.Run("Nested test", func(t *testing.T) {
			t.Run("Passes", func(t *testing.T) {
				testFunc := func(ft *functional.T) {
					ft.Run("Subtest 1", func(_ *functional.T) {
					})
				}
				pass := functional.Run(ctx, testFunc)
				assert.True(t, pass)
			})

			t.Run("Fails", func(t *testing.T) {
				testFunc := func(ft *functional.T) {
					ft.Run("Subtest 1", func(ft *functional.T) {
						ft.FailNow()
					})
				}
				pass := functional.Run(ctx, testFunc)
				assert.False(t, pass)
			})
		})

		t.Run("WithWriter()", func(t *testing.T) {
			testFunc := func(ft *functional.T) {
				ft.Log("Foobar")
			}
			var buf bytes.Buffer
			twriter := bufio.NewWriter(&buf)
			pass := functional.Run(ctx, testFunc, functional.WithWriter(twriter))
			assert.True(t, pass)
			assert.Contains(t, "Foobar", buf.String())
		})

		t.Run("WithArgs()", func(t *testing.T) {
			args := []string{"A", "B", "C"}
			testFunc := func(ft *functional.T) {
				assert.Equal(t, args, ft.Args())
			}
			pass := functional.Run(ctx, testFunc, functional.WithArgs(args...))
			assert.True(t, pass)
		})
	})

	t.Run("RunSuite()", func(t *testing.T) {
		t.Run("Happy path", func(t *testing.T) {
			var counter int
			testFunc1 := func(_ *functional.T) {
				counter++
			}
			testFunc2 := func(_ *functional.T) {
				counter++
			}
			tests := []functional.TestFunc{testFunc1, testFunc2}
			pass := functional.RunSuite(ctx, "Foobar suite", tests)
			assert.True(t, pass)
			assert.Equal(t, 2, counter)
		})

		t.Run("Partial failure", func(t *testing.T) {
			var counter int
			testFunc1 := func(_ *functional.T) {
				counter++
			}
			testFunc2 := func(ft *functional.T) {
				counter++
				ft.FailNow()
			}
			tests := []functional.TestFunc{testFunc1, testFunc2}
			pass := functional.RunSuite(ctx, "Foobar suite", tests)
			assert.False(t, pass)
			assert.Equal(t, 2, counter)
		})

		t.Run("Complete failure", func(t *testing.T) {
			var counter int
			testFunc1 := func(ft *functional.T) {
				counter++
				ft.FailNow()
			}
			testFunc2 := func(ft *functional.T) {
				counter++
				ft.FailNow()
			}
			tests := []functional.TestFunc{testFunc1, testFunc2}
			pass := functional.RunSuite(ctx, "Foobar suite", tests)
			assert.False(t, pass)
			assert.Equal(t, 2, counter)
		})
	})

	t.Run("RunBenchmarkTimes()", func(t *testing.T) {
		t.Run("Happy path", func(t *testing.T) {
			testCases := []struct {
				Name  string
				N     int
				Delay time.Duration
			}{
				{Name: "Once", N: 1, Delay: 5 * time.Millisecond},
				{Name: "2x", N: 2, Delay: 5 * time.Millisecond},
				{Name: "100x", N: 100, Delay: 500 * time.Microsecond},
			}

			for _, testCase := range testCases {
				t.Run(testCase.Name, func(t *testing.T) {
					var counter int
					benchmarkFunc := func(fb *functional.B) {
						for i := 0; i < fb.N; i++ {
							counter++
							time.Sleep(testCase.Delay)
						}
					}
					result := functional.RunBenchmarkTimes(ctx, benchmarkFunc, testCase.N)
					assert.True(t, result.Pass)
					assert.NotZero(t, result.NsPerOp)
					assert.Equal(t, testCase.N, counter)
				})
			}
		})

		t.Run("Subtest has same N value", func(t *testing.T) {
			const expectedN = 100
			var counterFunc1, counterFunc2a int
			benchmarkFunc := func(fb *functional.B) {
				fb.Run("Func1", func(fb *functional.B) {
					for i := 0; i < fb.N; i++ {
						counterFunc1++
					}
				})
				fb.Run("Func2", func(fb *functional.B) {
					fb.Run("Func2a", func(fb *functional.B) {
						for i := 0; i < fb.N; i++ {
							counterFunc2a++
						}
					})
				})
			}

			result := functional.RunBenchmarkTimes(ctx, benchmarkFunc, expectedN)
			assert.True(t, result.Pass)
			assert.Equal(t, expectedN, counterFunc1)
			assert.Equal(t, expectedN, counterFunc2a)
		})

		t.Run("WithWriter()", func(t *testing.T) {
			testFunc := func(fb *functional.B) {
				fb.Log("Foobar")
			}
			var buf bytes.Buffer
			bwriter := bufio.NewWriter(&buf)
			result := functional.RunBenchmarkTimes(ctx, testFunc, 1, functional.WithWriter(bwriter))
			assert.True(t, result.Pass)
			assert.Contains(t, "Foobar", buf.String())
		})

		t.Run("WithArgs()", func(t *testing.T) {
			args := []string{"A", "B", "C"}
			testFunc := func(fb *functional.B) {
				assert.Equal(t, args, fb.Args())
			}
			result := functional.RunBenchmarkTimes(ctx, testFunc, 1, functional.WithArgs(args...))
			assert.True(t, result.Pass)
		})

		t.Run("Benchmark fails", func(t *testing.T) {
			testFunc := func(fb *functional.B) {
				fb.FailNow()
			}
			result := functional.RunBenchmarkTimes(ctx, testFunc, 1)
			assert.False(t, result.Pass)
		})

		t.Run("Nested benchmark", func(t *testing.T) {
			t.Run("Passes", func(t *testing.T) {
				benchFunc := func(fb *functional.B) {
					fb.Run("Subtest 1", func(_ *functional.B) {
					})
				}
				result := functional.RunBenchmarkTimes(ctx, benchFunc, 1)
				assert.True(t, result.Pass)
			})

			t.Run("Fails", func(t *testing.T) {
				benchFunc := func(fb *functional.B) {
					fb.Run("Subtest 1", func(b *functional.B) {
						b.FailNow()
					})
				}
				result := functional.RunBenchmarkTimes(ctx, benchFunc, 1)
				assert.False(t, result.Pass)
			})
		})
	})

	t.Run("RunBenchmarkSuiteTimes()", func(t *testing.T) {
		const times = 5

		t.Run("Happy path", func(t *testing.T) {
			var counter int
			testFunc1 := func(fb *functional.B) {
				counter += fb.N
			}
			testFunc2 := func(fb *functional.B) {
				counter += fb.N
			}
			tests := []functional.BenchmarkFunc{testFunc1, testFunc2}
			pass := functional.RunBenchmarkSuiteTimes(ctx, "Foobar suite", times, tests)
			assert.True(t, pass)
			assert.Equal(t, 2*times, counter)
		})

		t.Run("Partial failure", func(t *testing.T) {
			var counter int
			testFunc1 := func(fb *functional.B) {
				counter += fb.N
			}
			testFunc2 := func(fb *functional.B) {
				counter += fb.N
				fb.FailNow()
			}
			tests := []functional.BenchmarkFunc{testFunc1, testFunc2}
			pass := functional.RunBenchmarkSuiteTimes(ctx, "Foobar suite", times, tests)
			assert.False(t, pass)
			assert.Equal(t, 2*times, counter)
		})

		t.Run("Complete failure", func(t *testing.T) {
			var counter int
			testFunc1 := func(fb *functional.B) {
				counter += fb.N
				fb.FailNow()
			}
			testFunc2 := func(fb *functional.B) {
				counter += fb.N
				fb.FailNow()
			}
			tests := []functional.BenchmarkFunc{testFunc1, testFunc2}
			pass := functional.RunBenchmarkSuiteTimes(ctx, "Foobar suite", times, tests)
			assert.False(t, pass)
			assert.Equal(t, 2*times, counter)
		})
	})
}
