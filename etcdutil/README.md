## ETCD Leader Election
Use etcd for leader election if you have several instances of a service running in production
and you only want one of the service instances to preform a task.

`LeaderElection` starts a goroutine which performs an election and maintains a leader
 while services join and leave the election. Calling `Stop()` will `Concede()` leadership if
  we currently have it.

```go

import (
    "github.com/mailgun/holster"
    "github.com/mailgun/holster/election"
)

var wg holster.WaitGroup

// Start the goroutine and preform the election
leader, _ := election.NewElection("my-service", "", nil)

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
```

