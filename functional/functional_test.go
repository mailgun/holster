package functional_test

import (
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
					benchmarkFunc := func(b *functional.B) {
						for i := 0; i < b.N; i++ {
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

		t.Run("Benchmark fails", func(t *testing.T) {
			testFunc := func(ft *functional.T) {
				ft.FailNow()
			}
			pass := functional.Run(ctx, testFunc)
			assert.False(t, pass)
		})

		t.Run("Nested benchmark", func(t *testing.T) {
			t.Run("Passes", func(t *testing.T) {
				benchFunc := func(b *functional.B) {
					b.Run("Subtest 1", func(_ *functional.B) {
					})
				}
				result := functional.RunBenchmarkTimes(ctx, benchFunc, 1)
				assert.True(t, result.Pass)
			})

			t.Run("Fails", func(t *testing.T) {
				benchFunc := func(b *functional.B) {
					b.Run("Subtest 1", func(b *functional.B) {
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
			testFunc1 := func(b *functional.B) {
				counter += b.N
			}
			testFunc2 := func(b *functional.B) {
				counter += b.N
			}
			tests := []functional.BenchmarkFunc{testFunc1, testFunc2}
			pass := functional.RunBenchmarkSuiteTimes(ctx, "Foobar suite", times, tests)
			assert.True(t, pass)
			assert.Equal(t, 2*times, counter)
		})

		t.Run("Partial failure", func(t *testing.T) {
			var counter int
			testFunc1 := func(b *functional.B) {
				counter += b.N
			}
			testFunc2 := func(b *functional.B) {
				counter += b.N
				b.FailNow()
			}
			tests := []functional.BenchmarkFunc{testFunc1, testFunc2}
			pass := functional.RunBenchmarkSuiteTimes(ctx, "Foobar suite", times, tests)
			assert.False(t, pass)
			assert.Equal(t, 2*times, counter)
		})

		t.Run("Complete failure", func(t *testing.T) {
			var counter int
			testFunc1 := func(b *functional.B) {
				counter += b.N
				b.FailNow()
			}
			testFunc2 := func(b *functional.B) {
				counter += b.N
				b.FailNow()
			}
			tests := []functional.BenchmarkFunc{testFunc1, testFunc2}
			pass := functional.RunBenchmarkSuiteTimes(ctx, "Foobar suite", times, tests)
			assert.False(t, pass)
			assert.Equal(t, 2*times, counter)
		})
	})
}
