package collections_test

import (
	"testing"
	"time"

	"github.com/mailgun/holster/v4/collections"
	"github.com/stretchr/testify/assert"
)

func TestNewExpireCache(t *testing.T) {
	ec := collections.NewExpireCache(time.Millisecond * 100)

	ec.Add("one", "one")
	time.Sleep(time.Millisecond * 100)

	var runs int
	ec.Each(1, func(key interface{}, value interface{}) error {
		assert.Equal(t, key, "one")
		assert.Equal(t, value, "one")
		runs++
		return nil
	})
	assert.Equal(t, runs, 1)

	// Should NOT be in the cache
	time.Sleep(time.Millisecond * 100)
	ec.Each(1, func(key interface{}, value interface{}) error {
		runs++
		return nil
	})
	assert.Equal(t, runs, 1)
}
