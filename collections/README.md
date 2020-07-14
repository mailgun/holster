## LRUCache
Implements a Least Recently Used Cache with optional TTL and stats collection

This is a LRU cache based off [github.com/golang/groupcache/lru](https://github.com/golang/groupcache/tree/master/lru) expanded
with the following

* `Peek()` - Get the value without updating the expiration or last used or stats
* `Keys()` - Get a list of keys at this point in time
* `Stats()` - Returns stats about the current state of the cache
* `AddWithTTL()` - Adds a value to the cache with a expiration time
* `Each()` - Concurrent non blocking access to each item in the cache
* `Map()` - Efficient blocking modification to each item in the cache

TTL is evaluated during calls to `.Get()` if the entry is past the requested TTL `.Get()`
removes the entry from the cache counts a miss and returns not `ok`

```go
cache := collections.NewLRUCache(5000)
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
has been processed before removal. This cache is useful if you need an
unbounded queue, that can also act like an LRU cache.

Every time an item is touched by `.Get()` or `.Set()` the duration is
updated which ensures items in frequent use stay in the cache. Processing
the cache with `.Each()` can modify the item in the cache without
updating the expiration time by using the `.Update()` method.

The cache can also return statistics which can be used to graph cache usage
and size.

*NOTE: Because this is an unbounded cache, the user MUST process the cache
with `.Each()` regularly! Else the cache items will never expire and the cache
will eventually eat all the memory on the system*

```go
// How often the cache is processed
syncInterval := time.Second * 10

// In this example the cache TTL is slightly less than the sync interval
// such that before the first sync; items that where only accessed once
// between sync intervals should expire. This technique is useful if you
// have a long syncInterval and are only interested in keeping items
// that where accessed during the sync cycle
cache := collections.NewExpireCache((syncInterval / 5) * 4)

go func() {
    for {
        select {
        // Sync the cache with the database every 10 seconds
        // Items in the cache will not be expired until this completes without error
        case <-time.Tick(syncInterval):
            // Each() uses FanOut() to run several of these concurrently, in this
            // example we are capped at running 10 concurrently, Use 0 or 1 if you
            // don't need concurrent FanOut
            cache.Each(10, func(key inteface{}, value interface{}) error {
                item := value.(Item)
                return db.ExecuteQuery("insert into tbl (id, field) values (?, ?)",
                    item.Id, item.Field)
            })
        // Periodically send stats about the cache
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

## Priority Queue
Provides a Priority Queue implementation as described [here](https://en.wikipedia.org/wiki/Priority_queue)

```go
queue := collections.NewPriorityQueue()

queue.Push(&collections.PQItem{
    Value: "thing3",
    Priority: 3,
})

queue.Push(&collections.PQItem{
    Value: "thing1",
    Priority: 1,
})

queue.Push(&collections.PQItem{
    Value: "thing2",
    Priority: 2,
})

// Pops item off the queue according to the priority instead of the Push() order
item := queue.Pop()

fmt.Printf("Item: %s", item.Value.(string))

// Output: Item: thing1
```
