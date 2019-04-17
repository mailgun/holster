package etcdutil

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync/atomic"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/mailgun/holster"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var log *logrus.Entry

type LeaderElector interface {
	Start(context.Context) error
	LeaderChan() chan bool
	IsLeader() bool
	Concede() bool
	Stop()
}

type ElectionObserver func(bool, string)

type Election struct {
	conf      ElectionConfig
	client    *etcd.Client
	cancel    context.CancelFunc
	wg        holster.WaitGroup
	ctx       context.Context
	isLeader  int32
	observers []ElectionObserver
	session   *Session
}

type ElectionConfig struct {
	// The name of the election (IE: scout, blackbird, etc...)
	Election string
	// The name of this instance (IE: worker-n01, worker-n02, etc...)
	Candidate string
	// Seconds to wait before giving up the election if leader disconnected
	TTL int64
	// The size of the leader channel buffer as returned by LeaderChan(). Set this to
	// something other than zero to avoid losing leadership changes.
	LeaderChannelSize int
	// The logger entry to be used
	Log *logrus.Entry
}

// Use leader election if you have several instances of a service running in production
// and you only want one of the service instances to preform a periodic task.
//
//  client, _ := etcdutil.NewClient(nil)
//
//  election := etcdutil.NewElection(client, etcdutil.ElectionConfig{
//      Election: "election-name",
//      Candidate: "",
//      TTL: 5,
//  })
//
//  // Start the leader election and attempt to become leader
//  if err := election.Start(ctx); err != nil {
//      panic(err)
//  }
//
//	// Returns true if we are leader (thread safe)
//	if election.IsLeader() {
//		// Do periodic thing
//	}
//
//  select {
//  case isLeader := <-election.LeaderChan():
//  	fmt.Printf("Leader: %t\n", isLeader)
//  }
//
// NOTE: If this instance is elected leader and connection is interrupted to etcd,
// this library will continue to report it is leader until connection to etcd is resumed
// and a new leader is elected. If you wish to lose leadership on disconnect set
// `LoseLeaderOnDisconnect = true`
func NewElection(client *etcd.Client, conf ElectionConfig) *Election {
	log = logrus.WithField("category", "election")
	e := &Election{
		conf:   conf,
		client: client,
	}

	// Default to short 5 second leadership TTL
	holster.SetDefault(&e.conf.TTL, 5)
	e.conf.Election = path.Join("/elections", e.conf.Election)

	// Use the hostname if no candidate name provided
	if host, err := os.Hostname(); err == nil {
		holster.SetDefault(&e.conf.Candidate, host)
	}
	return e
}

// Register adds an election observer
func (e *Election) Register(obs ElectionObserver) {
	// TODO: Add a mutex or refuse to add or delete if election has already started
	e.observers = append(e.observers, obs)
}

func (e *Election) observeSession(leaseID etcd.LeaseID) {
	fmt.Printf("Lease ID: %s\n", leaseID)
}

func (e *Election) Start(ctx context.Context) error {
	if e.conf.Election == "" {
		return errors.New("ElectionConfig.Election can not be empty")
	}

	// Create a new Session
	var err error
	if e.session, err = NewSession(ctx, e.client, SessionConfig{
		Observer: e.observeSession,
		Log:      e.conf.Log,
		TTL:      e.conf.TTL,
	}); err != nil {
		return err
	}

	e.ctx, e.cancel = context.WithCancel(context.Background())

	/*ready := make(chan ElectionEvent, 5)
	e.Register("__internal", func(e ElectionEvent){
		ready <- e
	})
	defer e.UnRegister("__internal")*/

	// Monitor the status of our Session
	/*e.wg.Until(func(done chan struct{}) bool {
		select {
			case event := <- e.eventChan:
				switch event.Event {
				case EventLostLease:
					// TODO: Shutdown any active watchers and cancel our leadership if we have it
				case EventGainLease:
						// TODO: Loop until we gain leader or find out who is leader

				//if lease == nil {
					// Revoke leader if we have it
					if _, err := e.Concede(); err != nil {
						e.emitLog(err, "while conceding campaign '%s' after losing the heartbeat",
							e.conf.Candidate)
					}

					case <- e.
				} else {
					// Attempt to become leader
					if err := e.campaign(lease.ID); err != nil {
						e.emitLog(err, "while campaigning as '%s'", e.conf.Candidate)
					}
				}
		}
		return true
	})

	// TODO: Get the start index
	//s := &engine.Snapshot{Index: uint64(response.Header.Revision)}

	select {
	case e := <- ready:
		switch e.Event {
		case IsLeaderEvent, NotLeaderEvent:
			// TODO: Start watcher
			return nil
		}
		case <- ctx.Done():
			// TODO: Shutdown any running routinues
			return ctx.Err()
	}*/
	return nil
}

func (e *Election) campaign(l etcd.LeaseID) error {
	// Simple transaction on a key. The first one to create the key is the leader.
	txn := e.client.Txn(e.ctx).If(etcd.Compare(etcd.CreateRevision(e.conf.Election), "=", 0))
	txn = txn.Then(etcd.OpPut(e.conf.Election, e.conf.Candidate, etcd.WithLease(l)))
	txn = txn.Else(etcd.OpGet(e.conf.Election))
	resp, err := txn.Commit()
	if err != nil {
		return err
	}

	// We are leader
	if resp.Succeeded {
		e.emit(ElectionEvent{
			Event:    EventGainLeader,
			Revision: resp.Header.Revision,
			Msg:      e.conf.Candidate,
		})
		return nil
	}
	kv := resp.Responses[0].GetResponseRange().Kvs[0]

	// If we got disconnected previously and we are still leader
	// NOTE: The other possibility is that another candidate has our name, in this
	// case we are stuck in a loop until we are canceled by the context
	if string(kv.Value) == e.conf.Candidate {
		if _, err := e.Concede(); err != nil {
			return err
		}
		return fmt.Errorf("current leader matches our "+
			"candidate name '%s'; forcing a new election", e.conf.Candidate)
	}

	// We are not leader
	e.emit(ElectionEvent{
		Event:    EventLostLeader,
		Revision: kv.CreateRevision,
		Msg:      string(kv.Value),
	})
	return nil
}

func (e *Election) watch(ctx context.Context, afterIdx int) error {
	var watchChan etcd.WatchChan
	ready := make(chan struct{})

	watcher := etcd.NewWatcher(e.client)
	defer watcher.Close()

	go func() {
		watchChan = watcher.Watch(etcd.WithRequireLeader(ctx), e.conf.Election,
			etcd.WithRev(int64(afterIdx)), etcd.WithPrefix())
		close(ready)
	}()

	select {
	case <-ready:
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "while waiting for etcd watch to start")
	}

	e.wg.Until(func(done chan struct{}) bool {
		for response := range watchChan {
			if response.Canceled {
				log.Infof("watch cancelled")
				return false
			}
			if err := response.Err(); err != nil {
				log.Errorf("watch error: %v", err)
				return false
			}

			// TODO: Handle changes in leadership
			/*for _, event := range response.Events {
				log.Infof("%s", eventToString(event))
				change, err := n.parseChange(event)
				if err != nil {
					log.Warningf("ignore '%s', error: %s", eventToString(event), err)
					continue
				}
				if change != nil {
					log.Infof("%v", change)
					select {
					case changes <- change:
					case <-cancelC:
						return nil
					}
				}
			}*/
			select {
			case <-done:
				break
			}
		}
		return false
	})

	// TODO:
	//   e.notifyLeaderChange(false)
	//   Reset leadership if we had it previously
	//   e.setLeader(false)

	// TODO: Wait until we have a leader before returning

	return nil
}

func (e *Election) Stop() {
	e.Concede()
	e.cancel()
	e.wg.Wait()
	close(e.eventChan)
}

func (e *Election) IsLeader() bool {
	return atomic.LoadInt32(&e.isLeader) == 1
}

/*func (e *Election) LeaderChan() chan bool {
	return e.eventChan
}*/

// Release leadership and return true if we own it, else do nothing and return false
func (e *Election) Concede() (bool, error) {
	if atomic.LoadInt32(&e.isLeader) == 1 {
		// If resign takes longer than our TTL then lease is expired and we are no longer leader anyway.
		_, cancel := context.WithTimeout(e.ctx, time.Duration(e.conf.TTL)*time.Second)
		/*if err := e.election.Resign(ctx); err != nil {
			return true, err
		}*/
		cancel()
		e.setLeader(false)
		return true, nil
	}
	return false, nil
}

func (e *Election) setLeader(set bool) {
	var requested int32
	if set {
		requested = 1
	}

	// Only notify if leadership changed
	if requested == atomic.LoadInt32(&e.isLeader) {
		return
	}

	atomic.StoreInt32(&e.isLeader, requested)
}

func (e *Election) emitLog(err error, msg string, args ...interface{}) {
	logMsg := fmt.Sprintf(msg, args...)
	if err != nil {
		e.emit(ElectionEvent{
			Event: EventErr,
			Msg:   logMsg,
			Err:   err,
		})
		return
	}
	e.emit(ElectionEvent{
		Event: EventInfo,
		Msg:   logMsg,
	})
}

func (e *Election) emit(event ElectionEvent) {
	for _, obs := range e.observers {
		obs(event)
	}
}

/*func (e *Election) notifyLeaderChange(set bool) {
}

type LeaderElectionMock struct{}

func (s *LeaderElectionMock) IsLeader() bool        { return true }
func (s *LeaderElectionMock) LeaderChan() chan bool { return nil }
func (s *LeaderElectionMock) Concede() bool         { return true }
func (s *LeaderElectionMock) Start() error          { return nil }
func (s *LeaderElectionMock) Stop()                 {}*/
