# Holster
A place to holster mailgun's golang libraries and tools

## Installation

To use, run the following command: 
```bash
go get github.com/mailgun/holster/v4
```

## Clock
A drop in (almost) replacement for the system `time` package to make scheduled
events deterministic in tests. See the [clock readme](https://github.com/mailgun/holster/blob/master/clock/README.md) for details

## HttpSign
HttpSign is a library for signing and authenticating HTTP requests between web services.
See the [httpsign readme](https://github.com/mailgun/holster/blob/master/httpsign/README.md) for details

## Distributed Election
A distributed election implementation using etcd to coordinate elections
See the [etcd v3 readme](https://github.com/mailgun/holster/blob/master/etcdutil/README.md) for details

## Errors
Errors is a fork of [https://github.com/pkg/errors](https://github.com/pkg/errors) with additional
 functions for improving the relationship between structured logging and error handling in go
See the [errors readme](https://github.com/mailgun/holster/blob/master/errors/README.md) for details

## WaitGroup
Waitgroup is a simplification of `sync.Waitgroup` with item and error collection included.

Running many short term routines over a collection with `.Run()`
```go
import "github.com/mailgun/holster/v4/syncutils"
var wg syncutils.WaitGroup
for _, item := range items {
    wg.Run(func(item interface{}) error {
        // Do some long running thing with the item
        fmt.Printf("Item: %+v\n", item.(MyItem))
        return nil
    }, item)
}
errs := wg.Wait()
if errs != nil {
    fmt.Printf("Errs: %+v\n", errs)
}
```

Clean up long running routines easily with `.Loop()`
```go
import "github.com/mailgun/holster/v4/syncutils"
pipe := make(chan int32, 0)
var wg syncutils.WaitGroup
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

Loop `.Until()` `.Stop()` is called
```go
import "github.com/mailgun/holster/v4/syncutils"
var wg syncutils.WaitGroup

wg.Until(func(done chan struct{}) bool {
    select {
    case <- time.Tick(time.Second):
        // Do some periodic thing
    case <- done:
        return false
    }
    return true
})

// Close the done channel and wait for the routine to exit
wg.Stop()
```

## FanOut
FanOut spawns a new go-routine each time `.Run()` is called until `size` is reached,
subsequent calls to `.Run()` will block until previously `.Run()` routines have completed.
Allowing the user to control how many routines will run simultaneously. `.Wait()` then
collects any errors from the routines once they have all completed. FanOut allows you
to control how many goroutines spawn at a time while WaitGroup will not.

```go
import "github.com/mailgun/holster/v4/syncutils"
// Insert records into the database 10 at a time
fanOut := syncutils.NewFanOut(10)
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
* `Each()` - Concurrent non blocking access to each item in the cache
* `Map()` - Efficient blocking modification to each item in the cache

TTL is evaluated during calls to `.Get()` if the entry is past the requested TTL `.Get()`
removes the entry from the cache counts a miss and returns not `ok`

```go
import "github.com/mailgun/holster/v4/collections"
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
import "github.com/mailgun/holster/v4/collections"
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
            cache.Each(10, func(key interface{}, value interface{}) error {
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

## TTLMap
Provides a threadsafe time to live map useful for holding a bounded set of key'd values
 that can expire before being accessed. The expiration of values is calculated
 when the value is accessed or the map capacity has been reached.
```go
import "github.com/mailgun/holster/v4/collections"
ttlMap := collections.NewTTLMap(10)
clock.Freeze(time.Now())

// Set a value that expires in 5 seconds
ttlMap.Set("one", "one", 5)

// Set a value that expires in 10 seconds
ttlMap.Set("two", "twp", 10)

// Simulate sleeping for 6 seconds
clock.Sleep(time.Second * 6)

// Retrieve the expired value and un-expired value
_, ok1 := ttlMap.Get("one")
_, ok2 := ttlMap.Get("two")

fmt.Printf("value one exists: %t\n", ok1)
fmt.Printf("value two exists: %t\n", ok2)

// Output: value one exists: false
// value two exists: true
```

## Default values
These functions assist in determining if values are the golang default
 and if so, set a value
```go
import "github.com/mailgun/holster/v4/setter"
var value string

// Returns true if 'value' is zero (the default golang value)
setter.IsZero(value)

// Returns true if 'value' is zero (the default golang value)
setter.IsZeroValue(reflect.ValueOf(value))

// If 'dest' is empty or of zero value, assign the default value.
// This panics if the value is not a pointer or if value and
// default value are not of the same type.
var config struct {
    Foo string
    Bar int
}
setter.SetDefault(&config.Foo, "default")
setter.SetDefault(&config.Bar, 200)

// Supply additional default values and SetDefault will
// choose the first default that is not of zero value
setter.SetDefault(&config.Foo, os.Getenv("FOO"), "default")

// Use 'SetOverride() to assign the first value that is not empty or of zero
// value.  The following will override the config file if 'foo' is provided via
// the cli or defined in the environment.

loadFromFile(&config)
argFoo = flag.String("foo", "", "foo via cli arg")

setter.SetOverride(&config.Foo, *argFoo, os.Env("FOO"))
```

## Check for Nil interface
```go
func NewImplementation() MyInterface {
    // Type and Value are not nil
    var p *MyImplementation = nil
    return p
}

thing := NewImplementation()
assert.False(t, thing == nil)
assert.True(t, setter.IsNil(thing))
assert.False(t, setter.IsNil(&MyImplementation{}))
```

## GetEnv
Get a value from an environment variable or return the provided default
```go
import "github.com/mailgun/holster/v4/config"

var conf = sandra.CassandraConfig{
   Nodes:    []string{config.GetEnv("CASSANDRA_ENDPOINT", "127.0.0.1:9042")},
   Keyspace: "test",
}
```

## Random Things
A set of functions to generate random domain names and strings useful for testing

```go
// Return a random string 10 characters long made up of runes passed
util.RandomRunes("prefix-", 10, util.AlphaRunes, holster.NumericRunes)

// Return a random string 10 characters long made up of Alpha Characters A-Z, a-z
util.RandomAlpha("prefix-", 10)

// Return a random string 10 characters long made up of Alpha and Numeric Characters A-Z, a-z, 0-9
util.RandomString("prefix-", 10)

// Return a random item from the list given
util.RandomItem("foo", "bar", "fee", "bee")

// Return a random domain name in the form "random-numbers.[gov, net, com, ..]"
util.RandomDomainName()
```

## GoRoutine ID
Get the go routine id (useful for logging)
```go
import "github.com/mailgun/holster/v4/callstack"
logrus.Infof("[%d] Info about this go routine", stack.GoRoutineID())
```

## ContainsString
Checks if a given slice of strings contains the provided string.
If a modifier func is provided, it is called with the slice item before the comparation.
```go
import "github.com/mailgun/holster/v4/slice"

haystack := []string{"one", "Two", "Three"}
slice.ContainsString("two", haystack, strings.ToLower) // true
slice.ContainsString("two", haystack, nil) // false
```

## Priority Queue
Provides a Priority Queue implementation as described [here](https://en.wikipedia.org/wiki/Priority_queue)

```go
import "github.com/mailgun/holster/v4/collections"
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

## Broadcaster
Allow the user to notify multiple goroutines of an event. This implementation guarantees every goroutine will wake
for every broadcast sent. In the event the goroutine falls behind and more broadcasts() are sent than the goroutine
has handled the broadcasts are buffered up to 10,000 broadcasts. Once the broadcast buffer limit is reached calls
 to broadcast() will block until goroutines consuming the broadcasts can catch up.
 
```go
import "github.com/mailgun/holster/v4/syncutil"
    broadcaster := syncutil.NewBroadcaster()
    done := make(chan struct{})
    var mutex sync.Mutex
    var chat []string

    // Start some simple chat clients that are responsible for
    // sending the contents of the []chat slice to their clients
    for i := 0; i < 2; i++ {
        go func(idx int) {
            var clientIndex int
            for {
                mutex.Lock()
                if clientIndex != len(chat) {
                    // Pretend we are sending a message to our client via a socket
                    fmt.Printf("Client [%d] Chat: %s\n", idx, chat[clientIndex])
                    clientIndex++
                    mutex.Unlock()
                    continue
                }
                mutex.Unlock()

                // Wait for more chats to be added to chat[]
                select {
                case <-broadcaster.WaitChan(string(idx)):
                case <-done:
                    return
                }
            }
        }(i)
    }

    // Add some chat lines to the []chat slice
    for i := 0; i < 5; i++ {
        mutex.Lock()
        chat = append(chat, fmt.Sprintf("Message '%d'", i))
        mutex.Unlock()

        // Notify any clients there are new chats to read
        broadcaster.Broadcast()
    }

    // Tell the clients to quit
    close(done)
```

## UntilPass
Functional test helper which will run a suite of tests until the entire suite
passes, or all attempts have been exhausted.

```go
import (
    "github.com/mailgun/holster/v4/testutil"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/assert"
)

func TestUntilPass(t *testing.T) {
    rand.Seed(time.Now().UnixNano())
    var value string

    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodPost {
            // Sleep some rand amount to time to simulate some
            // async process happening on the server
            time.Sleep(time.Duration(rand.Intn(10))*time.Millisecond)
            // Set the value
            value = r.FormValue("value")
        } else {
            fmt.Fprintln(w, value)
        }
    }))
    defer ts.Close()

    // Start the async process that produces a value on the server
    http.PostForm(ts.URL, url.Values{"value": []string{"batch job completed"}})

    // Keep running this until the batch job completes or attempts are exhausted
    testutil.UntilPass(t, 10, time.Millisecond*100, func(t testutil.TestingT) {
        r, err := http.Get(ts.URL)

        // use of `require` will abort the current test here and tell UntilPass() to
        // run again after 100 milliseconds
        require.NoError(t, err)

        // Or you can check if the assert returned true or not
        if !assert.Equal(t, 200, r.StatusCode) {
            return
        }

        b, err := ioutil.ReadAll(r.Body)
        require.NoError(t, err)

        assert.Equal(t, "batch job completed\n", string(b))
    })
}
```

## UntilConnect
Waits until the test can connect to the TCP/HTTP server before continuing the test
```go
import (
    "github.com/mailgun/holster/v4/testutil"
    "golang.org/x/net/nettest"
    "github.com/stretchr/testify/require"
)

func TestUntilConnect(t *testing.T) {
    ln, err := nettest.NewLocalListener("tcp")
    require.NoError(t, err)

    go func() {
        cn, err := ln.Accept()
        require.NoError(t, err)
        cn.Close()
    }()
    // Wait until we can connect, then continue with the test
    testutil.UntilConnect(t, 10, time.Millisecond*100, ln.Addr().String())
}
```

### Retry Until
Retries a function until the function returns error = nil or until the context is deadline is exceeded
```go
ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
defer cancel()
err := retry.Until(ctx, retry.Interval(time.Millisecond*10), func(ctx context.Context, att int) error {
    res, err := http.Get("http://example.com/get")
    if err != nil {
        return err
    }
    if res.StatusCode != http.StatusOK {
        return errors.New("expected status 200")
    }
    // Do something with the body
    return nil
})
if err != nil {
    panic(err)
}
```

Backoff functions provided

* `retry.Attempts(10, time.Millisecond*10)` retries up to `10` attempts
* `retry.Interval(time.Millisecond*10)` retries at an interval indefinitely or until context is cancelled
* `retry.ExponentialBackoff{ Min: time.Millisecond, Max: time.Millisecond * 100, Factor: 2}` retries
 at an exponential backoff interval. Can accept an optional `Attempts` which will limit the number of attempts


### Retry Async
Runs a function asynchronously and retries it until it succeeds, or the context is 
cancelled or `Stop()` is called. This is useful in distributed programming where
you know a remote thing will eventually succeed, but you need to keep trying until
the remote thing succeeds, or we are told to shutdown.

```go
ctx := context.Background()
async := retry.NewRetryAsync()

backOff := &retry.ExponentialBackoff{
    Min:      time.Millisecond,
    Max:      time.Millisecond * 100,
    Factor:   2,
    Attempts: 10,
}

id := createNewEC2("my-new-server")

async.Async(id, ctx, backOff, func(ctx context.Context, i int) error {
    // Waits for a new EC2 instance to be created then updates the config and exits
    if err := updateInstance(id, mySettings); err != nil {
        return err
    }
    return nil
})
// Wait for all the asyncs to complete
async.Wait()
```


### OpenTelemetry
Tracing tools using OpenTelemetry client SDK and Jaeger Tracing server.

See [tracing
readme](https://github.com/mailgun/holster/blob/master/tracing/README.md) for
details.

### Context Utilities
See package directory `ctxutil`.

Use functions `ctxutil.WithDeadline()`/`WithTimeout()` instead of the `context`
equivalents to log details of the deadline and source file:line where it was
set.  Must enable debug logging.
