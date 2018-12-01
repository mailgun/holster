package etcdutil

import (
	"context"
	"os"
	"path"
	"sync/atomic"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/mailgun/holster"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var log *logrus.Entry

type LeaderElector interface {
	IsLeader() bool
	Concede() bool
	Start() error
	Stop()
}

type Election struct {
	// The name of the election (IE: scout, blackbird, etc...)
	Election string
	// The name of this instance (IE: worker-n01, worker-n02, etc...)
	Candidate string
	// Seconds to wait before giving up the election if leader disconnected
	TTL int
	// Report not leader when etcd connection is interrupted
	LoseLeaderOnDisconnect bool

	session  *concurrency.Session
	election *concurrency.Election
	client   *etcd.Client
	cancel   context.CancelFunc
	wg       holster.WaitGroup
	ctx      context.Context
	isLeader int32
}

// Use leader election if you have several instances of a service running in production
// and you only want one of the service instances to preform a periodic task.
//
//	election, _ := etcdv3.NewElection("election-name", "", nil)
//
//  // Start the leader election and attempt to become leader
//  election.Start()
//
//	// Returns true if we are leader (thread safe)
//	if election.IsLeader() {
//		// Do periodic thing
//	}
//
// NOTE: If this instance is elected leader and connection is interrupted to etcd,
// this library will continue to report it is leader until connection to etcd is resumed
// and a new leader is elected. If you wish to lose leadership on disconnect set
// `LoseLeaderOnDisconnect = true`
func NewElection(election, candidate string, client *etcd.Client) (*Election, error) {
	log = logrus.WithField("category", "election")
	ctx, cancelFunc := context.WithCancel(context.Background())
	e := &Election{
		Candidate: candidate,
		Election:  election,
		TTL:       5,
		cancel:    cancelFunc,
		ctx:       ctx,
		client:    client,
	}

	if host, err := os.Hostname(); err == nil {
		holster.SetDefault(&e.Candidate, host)
	}

	// Set a prefix key for elections
	e.Election = path.Join("/elections", e.Election)

	// Test the client
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	_, err := e.client.Get(ctx, e.Election)
	if err != nil {
		return nil, errors.Wrap(err, "while connecting to etcd")
	}

	return e, nil
}

func (e *Election) newSession(id int64) error {
	var err error
	e.session, err = concurrency.NewSession(e.client, concurrency.WithTTL(e.TTL),
		concurrency.WithContext(e.ctx), concurrency.WithLease(etcd.LeaseID(id)))
	if err != nil {
		return errors.Wrap(err, "while creating new session")
	}
	e.election = concurrency.NewElection(e.session, e.Election)
	return nil
}

func (e *Election) Start() error {
	err := e.newSession(0)
	if err != nil {
		return errors.Wrap(err, "while creating new initial etcd session")
	}

	e.wg.Until(func(done chan struct{}) bool {
		var node *etcd.GetResponse
		var observe <-chan etcd.GetResponse

		// Get the current leader if any
		if node, err = e.election.Leader(e.ctx); err != nil {
			if err != concurrency.ErrElectionNoLeader {
				log.Errorf("while determining election leader: %s", err)
				goto reconnect
			}
		} else {
			// If we are resuming an election from which we previously had leadership we
			// have 2 options
			// 1. Resume the leadership if the lease has not expired. This is a race as the
			//    lease could expire in between the `Leader()` call and `Campaign()` calls.
			//    If this happens `Campaign()` should return with an error as the resumed
			//    session has expired.
			// 2. Resign the leadership immediately to allow a new leader to be chosen.
			//    This option will almost always result in transfer of leadership.
			//
			// We choose option 1 here because transfer of leadership might be expensive
			// for some users of this library and being a bit more chatty on reconnect can
			// be worth the price of switching leadership needlessly.

			// If we were the leader previously
			if string(node.Kvs[0].Value) == e.Candidate {
				// Recreate our session with the old lease
				if err = e.newSession(node.Kvs[0].Lease); err != nil {
					log.Errorf("while re-establishing session: %s", err)
					// abandon resuming leadership
					goto reconnect
				}
				e.election = concurrency.ResumeElection(e.session, e.Election,
					string(node.Kvs[0].Key), node.Kvs[0].CreateRevision)
			}
			// TODO: Sleep here to test race condition after resuming old session
		}

		// Reset leadership if we had it previously
		atomic.StoreInt32(&e.isLeader, 0)

		// Attempt to become leader
		if err := e.election.Campaign(e.ctx, e.Candidate); err != nil {
			if errors.Cause(err) == context.Canceled {
				return false
			}
			log.Debug("campaign err: %s\n", err)
			e.session.Close()
			goto reconnect
		}

		// If Campaign() returned without error, we are leader
		atomic.StoreInt32(&e.isLeader, 1)

		// Observe changes to leadership
		observe = e.election.Observe(e.ctx)
		for {
			select {
			case node, ok := <-observe:
				if !ok {
					log.Debug("observe chan closed\n")
					e.session.Close()
					goto reconnect
				}
				if string(node.Kvs[0].Value) == e.Candidate {
					log.Debug("IS Leader")
					atomic.StoreInt32(&e.isLeader, 1)
				} else {
					// We are not leader
					log.Debug("NOT Leader")
					atomic.StoreInt32(&e.isLeader, 0)
					return true
				}
			case <-e.ctx.Done():
				log.Debug("context cancelled")
				return false
			case <-e.session.Done():
				log.Debug("session cancelled\n")
				goto reconnect
			}
		}

	reconnect:
		if e.LoseLeaderOnDisconnect {
			atomic.StoreInt32(&e.isLeader, 0)
		}

		log.Debug("disconnected, creating new session")
		if err = e.newSession(0); err != nil {
			if errors.Cause(err) == context.Canceled {
				return false
			}
			log.Errorf("while creating new etcd session: %s", err)
		} else {
			log.Debug("New session created")
		}
		return true
	})

	// Wait until we have a leader before returning
	for {
		_, err := e.election.Leader(e.ctx)
		if err != nil {
			if err != concurrency.ErrElectionNoLeader {
				return err
			}
			time.Sleep(time.Millisecond * 300)
			continue
		}
		break
	}
	return nil
}

func (e *Election) Stop() {
	e.Concede()
	e.cancel()
	e.wg.Wait()
}

func (e *Election) IsLeader() bool {
	return atomic.LoadInt32(&e.isLeader) == 1
}

// Release leadership and return true if we own it, else do nothing and return false
func (e *Election) Concede() bool {
	if atomic.LoadInt32(&e.isLeader) == 1 {
		// If resign takes longer than our TTL then lease is expired and we are no longer leader anyway.
		ctx, cancel := context.WithTimeout(e.ctx, time.Duration(e.TTL)*time.Second)
		if err := e.election.Resign(ctx); err != nil {
			log.WithField("err", err).
				Error("while attempting to concede the election")
		}
		cancel()
		atomic.StoreInt32(&e.isLeader, 0)
		return true
	}
	return false
}

type LeaderElectionMock struct{}

func (s *LeaderElectionMock) IsLeader() bool { return true }
func (s *LeaderElectionMock) Concede() bool  { return true }
func (s *LeaderElectionMock) Start() error   { return nil }
func (s *LeaderElectionMock) Stop()          {}
