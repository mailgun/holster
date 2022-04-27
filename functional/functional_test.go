package functional_test

import (
	"context"
	"testing"

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
					ft.Run("Subtest 1", func(_ *functional.T) {
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
			var counter int64
			testFunc1 := func(ft *functional.T) {
				counter++
			}
			testFunc2 := func(ft *functional.T) {
				counter++
			}
			tests := []functional.TestFunc{testFunc1, testFunc2}
			pass := functional.RunSuite(ctx, "Foobar suite", tests)
			assert.True(t, pass)
			assert.Equal(t, int64(2), counter)
		})

		t.Run("Partial failure", func(t *testing.T) {
			var counter int64
			testFunc1 := func(ft *functional.T) {
				counter++
			}
			testFunc2 := func(ft *functional.T) {
				counter++
				ft.FailNow()
			}
			tests := []functional.TestFunc{testFunc1, testFunc2}
			pass := functional.RunSuite(ctx, "Foobar suite", tests)
			assert.False(t, pass)
			assert.Equal(t, int64(2), counter)
		})

		t.Run("Complete failure", func(t *testing.T) {
			var counter int64
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
			assert.Equal(t, int64(2), counter)
		})
	})
}
