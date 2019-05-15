package etcdutil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"sync/atomic"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/mailgun/holster"
	"github.com/pkg/errors"
)

type LeaderElector interface {
	IsLeader() bool
	Concede() (bool, error)
	Close()
}

var _ LeaderElector = &Election{}

type Event struct {
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

type EventObserver func(Event)

type Election struct {
	observers map[string]EventObserver
	backOff   *holster.BackOffCounter
	cancel    context.CancelFunc
	wg        holster.WaitGroup
	ctx       context.Context
	conf      ElectionConfig
	timeout   time.Duration
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
//		EventObserver: func(e etcdutil.Event) {
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
//  election.Close()
//
func NewElection(ctx context.Context, client *etcd.Client, conf ElectionConfig) (*Election, error) {
	if conf.Election == "" {
		return nil, errors.New("ElectionConfig.Election can not be empty")
	}

	// Default to short 5 second leadership TTL
	holster.SetDefault(&conf.TTL, int64(5))
	conf.Election = path.Join("/elections", conf.Election)

	// Use the hostname if no candidate name provided
	if host, err := os.Hostname(); err == nil {
		holster.SetDefault(&conf.Candidate, host)
	}

	e := &Election{
		backOff:   holster.NewBackOff(time.Millisecond*500, time.Duration(conf.TTL)*time.Second, 2),
		timeout:   time.Duration(conf.TTL) * time.Second,
		observers: make(map[string]EventObserver),
		client:    client,
		conf:      conf,
	}

	// Create a new Session
	var err error
	if e.session, err = NewSession(e.client, SessionConfig{
		Observer: e.onSessionChange,
		TTL:      e.conf.TTL,
	}); err != nil {
		return nil, err
	}
	e.ctx, e.cancel = context.WithCancel(context.Background())

	// If an observer was provided
	if conf.EventObserver != nil {
		e.observers["conf"] = conf.EventObserver
	}

	ready := make(chan struct{})
	// Register ourselves as an observer for the initial election, then remove before returning
	e.observers["init"] = func(event Event) {
		// If we get an error while waiting on the election results, pass that back to the caller
		if event.Err != nil {
			err = event.Err
		}
		delete(e.observers, "init")
		close(ready)
	}

	// Wait for results of leader election
	select {
	case <-ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return e, err
}

func (e *Election) onSessionChange(leaseID etcd.LeaseID, err error) {
	//logrus.Debugf("Lease ID: %v running: %t err: %v", leaseID, e.isRunning, err)

	// If we lost our lease, concede the campaign and stop
	if leaseID == NoLease {
		// Avoid stopping twice
		if !e.isRunning {
			return
		}
		e.wg.Stop()
		e.isRunning = false
		if err != nil {
			e.onErr(err, "lease error")
		}
		return
	}

	if e.isRunning {
		//logrus.Debugf("already running '%v", leaseID)
		return
	}

	e.isRunning = true

	e.wg.Until(func(done chan struct{}) bool {
		var err error
		var rev int64

		//logrus.Debug("registering")
		rev, err = e.registerCampaign(leaseID)
		if err != nil {
			e.onErr(err, "during campaign registration")
			select {
			case <-time.After(e.backOff.Next()):
				return true
			case <-done:
				return false
			}
		}
		e.backOff.Reset()

		//logrus.Debugf("watching rev %v", rev)
		if err := e.watchCampaign(rev); err != nil {
			e.onErr(err, "during campaign watch")
			select {
			case <-time.After(e.backOff.Next()):
				return true
			case <-done:
			}

			// If delete takes longer than our TTL then lease is expired
			// and we are no longer leader anyway.
			ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
			// Withdraw our candidacy since an error occurred
			if err := e.withDrawCampaign(ctx); err != nil {
				e.onErr(err, "")
			}
			cancel()
		}
		return false
	})
}

func (e *Election) withDrawCampaign(ctx context.Context) error {
	//logrus.Debugf("withDrawCampaign(%s)", e.key)
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
	e.key = fmt.Sprintf("%s%x", e.conf.Election, id)
	txn := e.client.Txn(e.ctx).If(etcd.Compare(etcd.CreateRevision(e.key), "=", 0))
	txn = txn.Then(etcd.OpPut(e.key, e.conf.Candidate, etcd.WithLease(id)))
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
		if string(kv.Value) != e.conf.Candidate {
			if _, err = e.client.Put(e.ctx, e.key, e.conf.Candidate); err != nil {
				return 0, err
			}
		}
	}
	return revision, nil
}

// getLeader returns a KV pair for the current leader
func (e *Election) getLeader(ctx context.Context) (*mvccpb.KeyValue, error) {
	// The leader is the first entry under the election prefix
	resp, err := e.client.Get(ctx, e.conf.Election, etcd.WithFirstCreate()...)
	if err != nil {
		return nil, err
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

	//logrus.Debugf("Current Leader %v", string(leaderKV.Key))

	watcher := etcd.NewWatcher(e.client)

	// We do this because watcher does not reliably return when errors occur on connect
	// or when cancelled (See https://github.com/etcd-io/etcd/pull/10020)
	go func() {
		//logrus.Debugf("watching prefix: %s", e.conf.Election)
		watchChan = watcher.Watch(etcd.WithRequireLeader(e.ctx), e.conf.Election,
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
		//logrus.Debug("Watching...")
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

			// Look for changes in leadership
			for _, event := range resp.Events {
				if event.Type == etcd.EventTypeDelete || event.Type == etcd.EventTypePut {
					// Skip events that are about us
					if string(event.Kv.Key) == e.key {
						continue
					}

					// If the key is for our current leader
					if bytes.Compare(event.Kv.Key, leaderKV.Key) == 0 {
						//logrus.Debug("Leader Changed")
						// Check our leadership status
						resp, err := e.getLeader(e.ctx)
						if err != nil {
							e.onFatalErr(err, "while querying for new leader")
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
			//logrus.Debug("done")
			watcher.Close()
			// If withdraw takes longer than our TTL then lease is expired
			// and we are no longer leader anyway.
			ctx, cancel := context.WithTimeout(context.Background(), e.timeout)

			// Withdraw our candidacy because of shutdown
			if err := e.withDrawCampaign(ctx); err != nil {
				e.onErr(err, "")
			}
			e.onLeaderChange(nil)
			cancel()
			return false
		}
		return true
	})
	return nil
}

func (e *Election) onLeaderChange(kv *mvccpb.KeyValue) {
	//logrus.Debug("onLeaderChange()")
	event := Event{}

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

	for _, v := range e.observers {
		v(event)
	}
}

// onErr reports errors the the observer
func (e *Election) onErr(err error, msg string) {
	atomic.StoreInt32(&e.isLeader, 0)

	if msg != "" {
		err = errors.Wrap(err, msg)
	}

	for _, v := range e.observers {
		v(Event{Err: err})
	}
}

// onFatalErr reports errors to the observer and resets the election and session
func (e *Election) onFatalErr(err error, msg string) {
	e.onErr(err, msg)
	// Cancel any campaigns and reset the session
	e.session.Reset(e.ctx)
}

// Close cancels the election and concedes the election if we are leader
func (e *Election) Close() {
	e.session.Close()
	e.wg.Wait()
}

// IsLeader returns true if we are leader
func (e *Election) IsLeader() bool {
	return atomic.LoadInt32(&e.isLeader) == 1
}

// Concede concedes leadership if we are leader and restarts the campaign returns true.
// if we are not leader do nothing and return false. If you want to concede leadership
// and cancel the campaign call Close() instead.
func (e *Election) Concede() (bool, error) {
	isLeader := atomic.LoadInt32(&e.isLeader)
	// If resign takes longer than our TTL then lease is expired and we are no longer leader anyway.
	ctx, cancel := context.WithTimeout(e.ctx, time.Duration(e.conf.TTL)*time.Second)

	defer func() {
		cancel()
		// Even if the delete fails we should consider ourselves no longer leader
		atomic.StoreInt32(&e.isLeader, 0)
	}()

	if _, err := e.client.Delete(ctx, e.key); err != nil {
		return isLeader == 1, err
	}
	return isLeader == 1, nil
}

type AlwaysLeaderMock struct{}

func (s *AlwaysLeaderMock) IsLeader() bool         { return true }
func (s *AlwaysLeaderMock) Concede() (bool, error) { return true, nil }
func (s *AlwaysLeaderMock) Close()                 {}
