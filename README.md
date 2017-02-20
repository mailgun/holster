# Holster
A place to put useful mailgun utilities and small libraries that don't fit anywhere else.

## FanOut
FanOut spawns a new go-routine each time `Run()` is called until `size` is reached,
subsequent calls to `Run()` will block until previously `Run()` routines have completed.
Allowing the user to control how many routines will run simultaneously. `Wait()` then
collects any errors from the routines once they have all completed.

```go
// Insert records into the database 10 at a time
fanOut := holster.NewFanOut(10)
for _, item := range items {
    fanOut.Run(func(cast interface{}) error {
        item := cast.(Item)
        return db.ExecuteQuery("insert into tbl (id, field) values (?, ?)",
            item.Id, item.Field)
    }, item)
}

// Collect errors
errs := fanOut.Wait()
if errs != nil {
	// do something with errs
}
```

## LRUCache
Implements a Least Recently Used Cache with optional TTL and stats collection

This is a LRU cache based off [github.com/golang/groupcache/lru](http://github.com/golang/groupcache/lru) expanded
with the following

* `Peek()` - Get the value without updating the expiration or last used or stats
* `Keys()` - Get a list of keys at this point in time
* `Stats()` - Returns stats about the current state of the cache
* `AddWithTTL()` - Adds a value to the cache with a expiration time

TTL is evaluated during calls to `Get()` if the entry is past the requested TTL `Get()`
removes the entry from the cache counts a miss and returns not `ok`

```go
cache := NewLRUCache(5000)
go func() {
    for {
        select {
        // Send cache stats every 5 seconds
        case <-time.Tick(time.Second * 5):
            stats := cache.GetStats()
            metrics.Gauge(metrics.Metric("demo", "cache", "size"), int64(stats.Size), 1)
            metrics.Gauge(metrics.Metric("demo", "cache", "hit"), stats.Hit, 1)
            metrics.Gauge(metrics.Metric("demo", "cache", "miss"), stats.Miss, 1)
        }
    }
}()

cache.Add("key", "value")
value, ok := cache.Get("key")

for _, key := range cache.Keys() {
    value, ok := cache.Get(key)
    if ok {
        fmt.Printf("Key: %+v Value %+v\n", key, value)
    }
}
```

## ExpireCache
ExpireCache is a cache which expires entries only after 2 conditions are met

1. The Specified TTL has expired
2. The item has been processed with ExpireCache.Each()

This is an unbounded cache which guaranties each item in the cache
has been processed before removal. This is different from a LRU
cache, as the cache might decide an item needs to be removed
(because we hit the cache limit) before the item has been processed.

Every time an item is touched by `Get()` or `Set()` the duration is
updated which ensures items in frequent use stay in the cache. Processing
the cache with `Each()` can modify the item in the cache without 
updating the expiration time by using the `Update()` method.

The cache can also return statistics which can be used to graph cache usage
and size.

NOTE: Because this is an unbounded cache, the user MUST process the cache
with `Each()` regularly! Else the cache items will never expire and the cache
will eventually eat all the memory on the system

```go
// How often the cache is processed
syncInterval := time.Second * 10

// In this example the cache TTL is slightly less than the sync interval
// such that before the first sync; items that where only accessed once
// between sync intervals should expire. This technique is useful if you
// have a long syncInterval and are only interested in keeping very 
// frequently used items
cache := holster.NewExpireCache((syncInterval / 5) * 4)

go func() {
    for {
        select {
        // Sync the cache with the database every 10 seconds
        // Items in the cache will not be expired until this completes without error
        case <-time.Tick(syncInterval):
            // Each() uses FanOut() to run several of these concurrently, in this
            // example we are capped at running 10 concurrently
            cache.Each(10, func(key inteface{}, value interface{}) error {
                item := value.(Item)
                return db.ExecuteQuery("insert into tbl (id, field) values (?, ?)",
                    item.Id, item.Field)
            })
        case <-time.Tick(time.Second * 5):
            stats := cache.GetStats()
            metrics.Gauge(metrics.Metric("demo", "cache", "size"), int64(stats.Size), 1)
            metrics.Gauge(metrics.Metric("demo", "cache", "hit"), stats.Hit, 1)
            metrics.Gauge(metrics.Metric("demo", "cache", "miss"), stats.Miss, 1)
        }
    }
}()

cache.Add("domain-id", Item{Id: 1, Field: "value"},
item, ok := cache.Get("domain-id")
if ok {
    fmt.Printf("%+v\n", item.(Item))
}
```
