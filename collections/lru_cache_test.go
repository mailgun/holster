/*
Copyright 2017 Mailgun Technologies Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This work is derived from github.com/golang/groupcache/lru
*/
package collections_test

import (
	"fmt"
	"testing"

	"github.com/mailgun/holster/v3/clock"
	"github.com/mailgun/holster/v3/collections"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLRUCache(t *testing.T) {
	cache := collections.NewLRUCache(5)

	// Confirm non existent key
	value, ok := cache.Get("key")
	assert.Nil(t, value)
	assert.Equal(t, false, ok)

	// Confirm add new value
	cache.Add("key", "value")
	value, ok = cache.Get("key")
	assert.Equal(t, "value", value)
	assert.Equal(t, true, ok)

	// Confirm overwrite current value correctly
	cache.Add("key", "new")
	value, ok = cache.Get("key")
	assert.Equal(t, "new", value)
	assert.Equal(t, true, ok)

	// Confirm removal works
	cache.Remove("key")
	value, ok = cache.Get("key")
	assert.Nil(t, value)
	assert.Equal(t, false, ok)

	// Stats should be correct
	stats := cache.Stats()
	assert.Equal(t, int64(2), stats.Hit)
	assert.Equal(t, int64(2), stats.Miss)
	assert.Equal(t, int64(0), stats.Size)
}

func TestLRUCacheWithTTL(t *testing.T) {
	cache := collections.NewLRUCache(5)
	start := clock.Now()
	clock.Freeze(start)

	cache.AddWithTTL("key", "value", 10*clock.Nanosecond)

	clock.Advance(10 * clock.Nanosecond)
	value, ok := cache.Get("key")
	assert.Equal(t, "value", value)
	assert.Equal(t, true, ok)

	clock.Advance(clock.Nanosecond)
	value, ok = cache.Get("key")
	assert.Nil(t, value)
	assert.Equal(t, false, ok)
}

func TestLRUCacheEach(t *testing.T) {
	cache := collections.NewLRUCache(5)

	cache.Add("1", 1)
	cache.Add("2", 2)
	cache.Add("3", 3)
	cache.Add("4", 4)
	cache.Add("5", 5)

	var count int
	// concurrency of 0, means no concurrency (This test will not develop a race condition)
	errs := cache.Each(0, func(key interface{}, value interface{}) error {
		count++
		return nil
	})
	assert.Nil(t, errs)
	assert.Equal(t, 5, count)

	stats := cache.Stats()
	assert.Equal(t, int64(0), stats.Hit)
	assert.Equal(t, int64(0), stats.Miss)
	assert.Equal(t, int64(5), stats.Size)
}

func TestLRUCacheMap(t *testing.T) {
	cache := collections.NewLRUCache(5)

	cache.Add("1", 1)
	cache.Add("2", 2)
	cache.Add("3", 3)
	cache.Add("4", 4)
	cache.Add("5", 5)

	var count int
	cache.Map(func(item *collections.CacheItem) bool {
		count++
		if v, ok := item.Value.(int); ok {
			// Remove value 3
			if v == 3 {
				return false
			}
		}
		return true
	})
	assert.Equal(t, 5, count)
	assert.Equal(t, 4, cache.Size())

	for _, item := range []int{1, 2, 4, 5} {
		v, ok := cache.Get(fmt.Sprintf("%d", item))
		require.True(t, ok)
		assert.Equal(t, item, v.(int))
	}
}
