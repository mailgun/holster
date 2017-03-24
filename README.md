# Holster
A place to put useful mailgun utilities and small libraries that don't fit anywhere else.

## Errors
Errors is a fork of [https://github.com/pkg/errors](https://github.com/pkg/errors) with additional
 functions for improving the relationship between structured logging and error handling in go
See the [errors readme](https://github.com/mailgun/holster/blob/master/errors/README.md) for details

## WaitGroup
Waitgroup is a simplification of `sync.Waitgroup` with item and error collection included.

Running many short term routines over a collection with `.Run()`
```go
var wg WaitGroup
for _, item := range items {
    wg.Run(func(item interface{}) error {
        // Do some long running thing with the item
        fmt.Printf("Item: %+v\n", item.(MyItem))
        return nil
    }, item)
}
errs := wg.Wait()
if errs != nil {
    fmt.Printf("Errs: %+v\n", errrs)
}
```

Clean up long running routines easily with `.Loop()`
```go
pipe := make(chan int32, 0)
var wg WaitGroup
var count int32

wg.Loop(func() bool {
    select {
    case inc, ok := <-pipe:
        // If the pipe was closed, return false
        if !ok {
            return false
        }
        atomic.AddInt32(&count, inc)
    }
    return true
})

// Feed the loop some numbers and close the pipe
pipe <- 1
pipe <- 5
pipe <- 10
close(pipe)

// Wait for the loop to exit
wg.Wait()
```

## FanOut
FanOut spawns a new go-routine each time `.Run()` is called until `size` is reached,
subsequent calls to `.Run()` will block until previously `.Run()` routines have completed.
Allowing the user to control how many routines will run simultaneously. `.Wait()` then
collects any errors from the routines once they have all completed. FanOut allows you
to control how many goroutines spawn at a time while WaitGroup will not.

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

TTL is evaluated during calls to `.Get()` if the entry is past the requested TTL `.Get()`
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
cache := holster.NewExpireCache((syncInterval / 5) * 4)

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

## Default values
These functions assist in determining if values are the golang default and if so, set a value
```go
var value string

// Returns true if 'value' is zero (the default golang value)
holster.IsZero(value)

// Returns true if 'value' is zero (the default golang value)
holster.IsZeroValue(reflect.ValueOf(value))

// If 'value' is empty or of zero value, assign the default value.
// This panics if the value is not a pointer or if value and
// default value are not of the same type.
var config struct {
    Foo string
    Bar int
}
holster.SetDefault(&config.Foo, "default")
holster.SetDefault(&config.Bar, 200)
```


## Random Things
A set of functions to generate random domain names and strings useful for testing

```go
// Return a random string 10 characters long made up of runes passed
holster.RandomRunes("prefix-", 10, holster.AlphaRunes, hoslter.NumericRunes)

// Return a random string 10 characters long made up of Alpha Characters A-Z, a-z
holster.RandomAlpha("prefix-", 10)

// Return a random string 10 characters long made up of Alpha and Numeric Characters A-Z, a-z, 0-9
holster.RandomString("prefix-", 10)

// Return a random item from the list given
holster.RandomItem("foo", "bar", "fee", "bee")

// Return a random domain name in the form "random-numbers.[gov, net, com, ..]"
holster.RandomDomainName()
```

## Logrus ToFields()
Recursively convert a deeply nested struct or map to `logrus.Fields` such that the result is safe for JSON encoding.
(IE: Ignore non marshallerable types like `func`)
```go
conf := struct {
   Endpoints []string
   CallBack  func([]byte) bool
   LogLevel  int
}
// Outputs the contents of the config struct along with the info message
logrus.WithFields(holster.ToFields(conf)).Info("Starting service")
```

## GoRoutine ID
Get the go routine id (useful for logging)
```go
import "github.com/mailgun/holster/stack"
logrus.Infof("[%d] Info about this go routine", stack.GoRoutineID())
```