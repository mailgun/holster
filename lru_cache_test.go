package holster

import (
	"testing"
	"time"

	. "gopkg.in/check.v1"
)

func TestCache(t *testing.T) { TestingT(t) }

type LRUCacheTestSuite struct{}

var _ = Suite(&LRUCacheTestSuite{})

func (s *LRUCacheTestSuite) SetUpSuite(c *C) {
}

func (s *LRUCacheTestSuite) TestCache(c *C) {
	cache := NewLRUCache(5)

	// Confirm non existent key
	value, ok := cache.Get("key")
	c.Assert(value, IsNil)
	c.Assert(ok, Equals, false)

	// Confirm add new value
	cache.Add("key", "value")
	value, ok = cache.Get("key")
	c.Assert(value, Equals, "value")
	c.Assert(ok, Equals, true)

	// Confirm overwrite current value correctly
	cache.Add("key", "new")
	value, ok = cache.Get("key")
	c.Assert(value, Equals, "new")
	c.Assert(ok, Equals, true)

	// Confirm removal works
	cache.Remove("key")
	value, ok = cache.Get("key")
	c.Assert(value, IsNil)
	c.Assert(ok, Equals, false)

	// Stats should be correct
	stats := cache.Stats()
	c.Assert(stats.Hit, Equals, int64(2))
	c.Assert(stats.Miss, Equals, int64(2))
	c.Assert(stats.Size, Equals, int64(0))
}

func (s *LRUCacheTestSuite) TestCacheWithTTL(c *C) {
	cache := NewLRUCache(5)

	cache.AddWithTTL("key", "value", time.Nanosecond)
	value, ok := cache.Get("key")
	c.Assert(value, Equals, nil)
	c.Assert(ok, Equals, false)
}

func (s *LRUCacheTestSuite) TestCacheEach(c *C) {
	cache := NewLRUCache(5)

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
	c.Assert(count, Equals, 5)
	c.Assert(errs, IsNil)

	stats := cache.Stats()
	c.Assert(stats.Hit, Equals, int64(0))
	c.Assert(stats.Miss, Equals, int64(0))
	c.Assert(stats.Size, Equals, int64(5))
}
