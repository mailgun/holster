## NewElection()
Use etcd for leader election if you have several instances of a service running in production
and you only want one of the service instances to preform a task.

`LeaderElection` starts a goroutine which performs an election and maintains a leader
while services join and leave the election. Calling `Stop()` will `Concede()` leadership if
the service currently has it.

```go

import (
    "github.com/mailgun/holster"
    "github.com/mailgun/holster/etcdutil"
)

func main() {
    var wg holster.WaitGroup

    hostname, err := os.Hostname()
    if err != nil {
        fmt.Fprintf(os.Stderr, "while obtaining hostname: %s\n", err)
        return
    }

    // Preform an election called 'my-service' with hostname as the candidate name
    leader, _ := etcdutil.NewElection("my-service", hostname, nil)

    // Handle graceful shutdown
    signalChan := make(chan os.Signal, 1)
    signal.Notify(signalChan, os.Interrupt, os.Kill)

    // Do periodic thing
    tick := time.NewTicker(time.Second * 2)
    wg.Loop(func() bool {
        select {
        case <-tick.C:
            // Are we currently leader?
            if leader.IsLeader() {
                err := DoThing()
                if err != nil {
                    // Have another instance DoThing(), we can't for some reason
                    leader.Concede()
                }
            }
            return true
        case <-signalChan:
            leader.Stop()
            return false
        }
    })
    wg.Wait()
}
```

## NewEtcdConfig()
Designed to be used in applications that share the same etcd config
and wish to reuse the same config throughout the application.

```go
import (
    "os"
    "fmt"

    "github.com/mailgun/holster/etcdutil"
)

func main() {
    // These environment variables provided by the environment,
    // we set them here to only to illustrate how `NewEtcdConfig()`
    // uses the environment to create a new etcd config
    os.Setenv("ETCD3_USER", "root")
    os.Setenv("ETCD3_PASSWORD", "rootpw")
    os.Setenv("ETCD3_ENDPOINT", "etcd-n01:2379,etcd-n02:2379,etcd-n03:2379")

    // These default to /etc/mailgun/ssl/localhost/etcd-xxx.pem if the files exist
    os.Setenv("ETCD3_TLS_CERT", "/path/to/etcd-cert.pem")
    os.Setenv("ETCD3_TLS_KEY", "/path/to/etcd-key.pem")
    os.Setenv("ETCD3_CA", "/path/to/etcd-ca.pem")

    // Create a new etc config from available environment variables
    cfg, err := etcdutil.NewEtcdConfig(nil)
    if err != nil {
        fmt.Fprintf(os.Stderr, "while creating etcd config: %s\n", err)
        return
    }

    // Use cfg to init scroll
    // Use cfg to init eventbus
    // Use cfg to init leader election
}
```

## NewSecureClient()
Just like `NewEtcdConfig()` but returns a connected etcd client for use by the
rest of the application.

```go
import (
    "os"
    "fmt"

    "github.com/mailgun/holster/etcdutil"
)

func main() {
    // Create a new etc client from available environment variables
    client, err := etcdutil.NewSecureClient(nil)
    if err != nil {
        fmt.Fprintf(os.Stderr, "while creating etcd client: %s\n", err)
        return
    }

    // Use client
}
```
