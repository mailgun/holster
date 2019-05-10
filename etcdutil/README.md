## NewElection()
Use etcd for leader election if you have several instances of a service running in production
and you only want one of the service instances to preform a task.

`LeaderElection` starts a goroutine which performs an election and maintains a leader
while candidates join and leave the election. Calling `Close()` will concede leadership if
the service currently has it and will withdraw the candidate from the election.

```go

import (
    "github.com/mailgun/holster"
    "github.com/mailgun/holster/etcdutil"
)

func main() {
    var wg holster.WaitGroup

    client, err := etcdutil.NewClient(nil)
    if err != nil {
        fmt.Fprintf(os.Stderr, "while creating etcd client: %s\n", err)
        return
    }
    
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

    // Start a leader election and attempt to become leader, only returns after
    // determining the current leader.
	election := etcdutil.NewElection(ctx, client, etcdutil.ElectionConfig{
		Election:                "my-service",
		Candidate:               "my-candidate",
		EventObserver: func(e etcdutil.Event) {
			leaderChan <- e
			if e.IsDone {
				close(leaderChan)
			}
		},
		TTL: 10,
	})

    // Handle graceful shutdown
    signalChan := make(chan os.Signal, 1)
    signal.Notify(signalChan, os.Interrupt, os.Kill)

    // Do periodic thing
    tick := time.NewTicker(time.Second * 2)
    wg.Loop(func() bool {
        select {
        case <-tick.C:
            // Are we currently leader?
            if election.IsLeader() {
                err := DoThing()
                if err != nil {
                    // Have another instance run DoThing()
                    // since we can't for some reason.
                    election.Concede()
                }
            }
            return true
        case <-signalChan:
            election.Stop()
            return false
        }
    })
    wg.Wait()
    
    // Or you can pipe events to a channel
    for leader := range leaderChan {
    	fmt.Printf("Leader: %v\n", leader)
    }
}
```

## NewConfig()
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
    // we set them here to only to illustrate how `NewConfig()`
    // uses the environment to create a new etcd config
    os.Setenv("ETCD3_USER", "root")
    os.Setenv("ETCD3_PASSWORD", "rootpw")
    os.Setenv("ETCD3_ENDPOINT", "etcd-n01:2379,etcd-n02:2379,etcd-n03:2379")

    // These default to /etc/mailgun/ssl/localhost/etcd-xxx.pem if the files exist
    os.Setenv("ETCD3_TLS_CERT", "/path/to/etcd-cert.pem")
    os.Setenv("ETCD3_TLS_KEY", "/path/to/etcd-key.pem")
    os.Setenv("ETCD3_CA", "/path/to/etcd-ca.pem")
    
    // Set this to force connecting with TLS, but without cert verification
    os.Setenv("ETCD3_SKIP_VERIFY", "true")

    // Create a new etc config from available environment variables
    cfg, err := etcdutil.NewConfig(nil)
    if err != nil {
        fmt.Fprintf(os.Stderr, "while creating etcd config: %s\n", err)
        return
    }
}
```

## NewClient()
Just like `NewConfig()` but returns a connected etcd client for use by the
rest of the application.

```go
import (
    "os"
    "fmt"

    "github.com/mailgun/holster/etcdutil"
)

func main() {
    // Create a new etc client from available environment variables
    client, err := etcdutil.NewClient(nil)
    if err != nil {
        fmt.Fprintf(os.Stderr, "while creating etcd client: %s\n", err)
        return
    }

    // Use client
}
```
