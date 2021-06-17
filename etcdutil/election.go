package etcdutil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"sync/atomic"
	"time"

	"github.com/mailgun/holster/v4/setter"
	"github.com/mailgun/holster/v4/syncutil"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/mvccpb"
	etcd "go.etcd.io/etcd/client/v3"
)

type LeaderElector interface {
	IsLeader() bool
	Concede() (bool, error)
	Close()
}

var _ LeaderElector = &Election{}

type ElectionEvent struct {
	// True if our candidate is leader
	IsLeader bool
	// True if the election is shutdown and
	// no further events will follow.
	IsDone bool
	// Holds the current leader key
	LeaderKey string
	// Hold the current leaders data
	LeaderData string
	// If not nil, contains an error encountered
	// while participating in the election.
	Err error
}

// Deprecated, use ElectionEvent instead
type Event = ElectionEvent

type EventObserver func(ElectionEvent)

type Election struct {
	observer  EventObserver
	election  string
	candidate string
	backOff   *backOffCounter
	cancel    context.CancelFunc
	wg        syncutil.WaitGroup
	ctx       context.Context
	ttl       time.Duration
	client    *etcd.Client
	session   *Session
	key       string
	isLeader  int32
	isRunning bool
}

type ElectionConfig struct {
	// Optional function when provided is called every time leadership changes or an error occurs
	EventObserver EventObserver
	// The name of the election (IE: scout, blackbird, etc...)
	Election string
	// The name of this instance (IE: worker-n01, worker-n02, etc...)
	Candidate string
	// Seconds to wait before giving up the election if leader disconnected
	TTL int64
}

// NewElection creates a new leader election and submits our candidate for leader.
//
//  client, _ := etcdutil.NewClient(nil)
//
//  // Start a leader election and attempt to become leader, only returns after
//  // determining the current leader.
//  election := etcdutil.NewElection(client, etcdutil.ElectionConfig{
//      Election: "presidental",
//      Candidate: "donald",
//		EventObserver: func(e etcdutil.ElectionEvent) {
//		  	fmt.Printf("Leader Data: %t\n", e.LeaderData)
//			if e.IsLeader {
//				// Do thing as leader
//			}
//		},
//      TTL: 5,
//  })
//
//	// Returns true if we are leader (thread safe)
//	if election.IsLeader() {
//		// Do periodic thing
//	}
//
//  // Concede the election if leader and cancel our candidacy
//  // for the election.
//  election.Stop()
//
func NewElection(ctx context.Context, client *etcd.Client, conf ElectionConfig) (*Election, error) {
	var initialElectionErr error
	readyCh := make(chan struct{})
	initialElection := true
	userObserver := conf.EventObserver
	// Wrap user's observer to intercept the initial election.
	conf.EventObserver = func(event ElectionEvent) {
		if userObserver != nil {
			userObserver(event)
		}
		if initialElection {
			initialElection = false
			initialElectionErr = event.Err
			close(readyCh)
			return
		}
	}
	e := NewElectionAsync(client, conf)
	// Wait for results of the initial leader election.
	select {
	case <-readyCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return e, errors.WithStack(initialElectionErr)
}

// NewElectionAsync creates a new leader election and submits our candidate for
// leader. It does not wait for the election to complete. The caller must
// provide an election event observer to monitor the election outcome.
//
//  client, _ := etcdutil.NewClient(nil)
//
//  // Start a leader election and returns immediately.
//  election := etcdutil.NewElectionAsync(client, etcdutil.ElectionConfig{
//      Election: "presidental",
//      Candidate: "donald",
//		EventObserver: func(e etcdutil.Event) {
//		  	fmt.Printf("Leader Data: %t\n", e.LeaderData)
//			if e.IsLeader {
//				// Do thing as leader
//			}
//		},
//      TTL: 5,
//  })
//
//  // Cancels the election and concedes the election if we are leader.
//  election.Stop()
//
func NewElectionAsync(client *etcd.Client, conf ElectionConfig) *Election {
	setter.SetDefault(&conf.Election, "null")
	conf.Election = path.Join("/elections", conf.Election)
	if host, err := os.Hostname(); err == nil {
		setter.SetDefault(&conf.Candidate, host)
	}
	setter.SetDefault(&conf.TTL, int64(5))

	ttlDuration := time.Duration(conf.TTL) * time.Second
	e := Election{
		observer:  conf.EventObserver,
		election:  conf.Election,
		candidate: conf.Candidate,
		ttl:       ttlDuration,
		backOff:   newBackOffCounter(500*time.Millisecond, ttlDuration, 2),
		client:    client,
	}
	e.ctx, e.cancel = context.WithCancel(context.Background())
	e.session = &Session{
		observer: e.onSessionChange,
		ttl:      e.ttl,
		backOff:  newBackOffCounter(500*time.Millisecond, ttlDuration, 2),
		client:   client,
	}
	e.session.start()
	return &e
}

func (e *Election) onSessionChange(leaseID etcd.LeaseID, err error) {
	//log.Debugf("SessionChange: Lease ID: %v running: %t err: %v", leaseID, e.isRunning, err)

	// If we lost our lease, concede the campaign and stop
	if leaseID == NoLease {
		// Avoid stopping twice
		if !e.isRunning {
			return
		}
		e.wg.Stop()
		e.isRunning = false
		atomic.StoreInt32(&e.isLeader, 0)
		if err != nil {
			e.onErr(err, "lease error")
		}
		return
	}

	if e.isRunning {
		return
	}

	e.isRunning = true

	e.wg.Until(func(done chan struct{}) bool {
		var err error
		var rev int64

		rev, err = e.registerCampaign(leaseID)
		if err != nil {
			e.onErr(err, "during campaign registration")
			select {
			case <-time.After(e.backOff.Next()):
				return true
			case <-done:
				e.isRunning = false
				return false
			}
		}

		if err := e.watchCampaign(rev); err != nil {
			e.onErr(err, "during campaign watch")
			select {
			case <-time.After(e.backOff.Next()):
				return true
			case <-done:
			}

			// If delete takes longer than our TTL then lease is expired
			// and we are no longer leader anyway.
			ctx, cancel := context.WithTimeout(context.Background(), e.ttl)
			// Withdraw our candidacy since an error occurred
			if err := e.withDrawCampaign(ctx); err != nil {
				e.onErr(err, "")
			}
			cancel()
			return true
		}
		e.backOff.Reset()
		return false
	})
}

func (e *Election) withDrawCampaign(ctx context.Context) error {
	defer func() {
		atomic.StoreInt32(&e.isLeader, 0)
	}()

	_, err := e.client.Delete(ctx, e.key)
	if err != nil {
		return errors.Wrapf(err, "while withdrawing campaign '%s'", e.key)
	}
	return nil
}

func (e *Election) registerCampaign(id etcd.LeaseID) (revision int64, err error) {
	// Create an entry under the election prefix with our lease ID as the key name
	e.key = fmt.Sprintf("%s%x", e.election, id)
	txn := e.client.Txn(e.ctx).If(etcd.Compare(etcd.CreateRevision(e.key), "=", 0))
	txn = txn.Then(etcd.OpPut(e.key, e.candidate, etcd.WithLease(id)))
	txn = txn.Else(etcd.OpGet(e.key))
	resp, err := txn.Commit()
	if err != nil {
		return 0, err
	}
	revision = resp.Header.Revision

	// This shouldn't happen, our session should always tell us if we disconnected and
	// etcd should have provided us with a unique lease id. If it does happen then
	// we should write our candidate name as the value and assume ownership
	if !resp.Succeeded {
		kv := resp.Responses[0].GetResponseRange().Kvs[0]
		revision = kv.CreateRevision
		if string(kv.Value) != e.candidate {
			if _, err = e.client.Put(e.ctx, e.key, e.candidate); err != nil {
				return 0, err
			}
		}
	}
	return revision, nil
}

// getLeader returns a KV pair for the current leader
func (e *Election) getLeader(ctx context.Context) (*mvccpb.KeyValue, error) {
	// The leader is the first entry under the election prefix
	resp, err := e.client.Get(ctx, e.election, etcd.WithFirstCreate()...)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	return resp.Kvs[0], nil
}

// watchCampaign monitors the status of the campaign and notifying any
// changes in leadership to the observer.
func (e *Election) watchCampaign(rev int64) error {
	var watchChan etcd.WatchChan
	ready := make(chan struct{})

	// Get the current leader of this election
	leaderKV, err := e.getLeader(e.ctx)
	if err != nil {
		return errors.Wrap(err, "while querying for current leader")
	}
	if leaderKV == nil {
		return errors.Wrap(err, "found no leader when watch began")
	}

	watcher := etcd.NewWatcher(e.client)

	// We do this because watcher does not reliably return when errors occur on connect
	// or when cancelled (See https://github.com/etcd-io/etcd/pull/10020)
	go func() {
		watchChan = watcher.Watch(etcd.WithRequireLeader(e.ctx), e.election,
			etcd.WithRev(int64(rev+1)), etcd.WithPrefix())
		close(ready)
	}()

	select {
	case <-ready:
	case <-e.ctx.Done():
		return errors.Wrap(e.ctx.Err(), "while waiting for etcd watch to start")
	}

	// Notify the observers of the current leader
	e.onLeaderChange(leaderKV)

	e.wg.Until(func(done chan struct{}) bool {
		select {
		case resp := <-watchChan:
			if resp.Canceled {
				e.onFatalErr(errors.New("remote server cancelled watch"), "during campaign watch")
				return false
			}
			if err := resp.Err(); err != nil {
				e.onFatalErr(err, "during campaign watch, remote server returned err")
				return false
			}

			// Watch for changes in leadership
			for _, event := range resp.Events {
				if event.Type == etcd.EventTypeDelete || event.Type == etcd.EventTypePut {
					// If the key is for our current leader
					if bytes.Compare(event.Kv.Key, leaderKV.Key) == 0 {
						// Check our leadership status
						resp, err := e.getLeader(e.ctx)
						if err != nil {
							e.onFatalErr(err, "while querying for new leader")
							return false
						}

						// If we have no leader
						if resp == nil {
							e.onFatalErr(err, "After etcd event no leader was found, restarting election")
							return false
						}
						// Notify if leadership has changed
						if bytes.Compare(resp.Key, leaderKV.Key) != 0 {
							leaderKV = resp
							e.onLeaderChange(leaderKV)
						}
					}
				}
			}
		case <-done:
			_ = watcher.Close()
			// If withdraw takes longer than our TTL then lease is expired
			// and we are no longer leader anyway.
			ctx, cancel := context.WithTimeout(context.Background(), e.ttl)

			// Withdraw our candidacy because of shutdown
			if err := e.withDrawCampaign(ctx); err != nil {
				e.onErr(err, "")
			}
			e.onLeaderChange(&mvccpb.KeyValue{})
			cancel()
			return false
		}
		return true
	})
	return nil
}

func (e *Election) onLeaderChange(kv *mvccpb.KeyValue) {
	event := ElectionEvent{}
	if kv != nil {
		if string(kv.Key) == e.key {
			atomic.StoreInt32(&e.isLeader, 1)
			event.IsLeader = true
		} else {
			atomic.StoreInt32(&e.isLeader, 0)
		}
		event.LeaderKey = string(kv.Key)
		event.LeaderData = string(kv.Value)
	} else {
		event.IsDone = true
	}
	if e.observer != nil {
		e.observer(event)
	}
}

// onErr reports errors the the observer
func (e *Election) onErr(err error, msg string) {
	atomic.StoreInt32(&e.isLeader, 0)
	if msg != "" {
		err = errors.Wrap(err, msg)
	}
	if e.observer != nil {
		e.observer(ElectionEvent{Err: err})
	}
}

// onFatalErr reports errors to the observer and resets the election and session
func (e *Election) onFatalErr(err error, msg string) {
	e.onErr(err, msg)
	// We call this in a go routine to avoid blocking on `Stop()` calls
	go e.session.Reset()
}

// Close cancels the election and concedes the election if we are leader
func (e *Election) Close() {
	e.session.Close()
	e.wg.Wait()
	// Emit the `Done:true` event
	e.onLeaderChange(nil)
}

// IsLeader returns true if we are leader. It only makes sense if the election
// was created with NewElection that block until the initial election is over.
func (e *Election) IsLeader() bool {
	return atomic.LoadInt32(&e.isLeader) == 1
}

// Concede concedes leadership if we are leader and restarts the campaign returns true.
// if we are not leader do nothing and return false. If you want to concede leadership
// and cancel the campaign call Close() instead.
func (e *Election) Concede() (bool, error) {
	isLeader := atomic.LoadInt32(&e.isLeader)
	if isLeader == 0 {
		return false, nil
	}
	oldCampaignKey := e.key
	e.session.Reset()

	// Ensure there are no lingering candidates
	ctx, cancel := context.WithTimeout(context.Background(), e.ttl)
	cancel()

	_, err := e.client.Delete(ctx, oldCampaignKey)
	if err != nil {
		return true, errors.Wrapf(err, "while cleaning up campaign '%s'", oldCampaignKey)
	}

	return true, nil
}

type AlwaysLeaderMock struct{}

func (s *AlwaysLeaderMock) IsLeader() bool         { return true }
func (s *AlwaysLeaderMock) Concede() (bool, error) { return true, nil }
func (s *AlwaysLeaderMock) Close()                 {}
