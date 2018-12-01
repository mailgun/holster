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

func (e *Election) newSession() (err error) {
	e.session, err = concurrency.NewSession(e.client, concurrency.WithTTL(e.TTL),
		concurrency.WithContext(e.ctx))
	if err != nil {
		return errors.Wrap(err, "while creating new session")
	}
	e.election = concurrency.NewElection(e.session, e.Election)
	return nil
}

func (e *Election) Start() error {
	err := e.newSession()
	if err != nil {
		return errors.Wrap(err, "while creating new initial etcd session")
	}

	e.wg.Until(func(done chan struct{}) bool {
		var node *etcd.GetResponse
		var observe <-chan etcd.GetResponse
		errChan := make(chan error)

		// Get the current leader if any
		node, err = e.election.Leader(e.ctx)
		if err != nil {
			if err != concurrency.ErrElectionNoLeader {
				log.Errorf("while determining election leader: %s", err)
				goto sleep
			}
		} else {
			// If we are resuming an election from which we previously had leadership
			// we should resign immediately to allow a new leader to be elected instead
			// of waiting for the lease to expire
			if string(node.Kvs[0].Value) == e.Candidate {
				// Existing election, resume the election
				election := concurrency.ResumeElection(e.session, e.Election,
					string(node.Kvs[0].Key), node.Kvs[0].CreateRevision)
				err = election.Resign(e.ctx)
				if err != nil {
					log.Errorf("while resigning a previous election: %s", err)
					goto sleep
				}
			}
		}

		// Reset leadership if we had it previously
		atomic.StoreInt32(&e.isLeader, 0)

		// Start a new campaign and attempt to become leader
		go func() {
			// This blocks indefinitely so place it in a goroutine
			errChan <- e.election.Campaign(e.ctx, e.Candidate)
		}()

		select {
		case err = <-errChan:
			if err != nil {
				if errors.Cause(err) == context.Canceled {
					return false
				}
				log.Errorf("while attempting to become leader: %s", err)
				e.session.Close()
				goto sleep
			}
		// Session was cancelled or we lost connection to etcd
		case <-e.session.Done():
			e.session.Close()
			goto sleep
		}

		// If Campaign() returned without error, we are leader
		atomic.StoreInt32(&e.isLeader, 1)

		// Observe changes to leadership
		observe = e.election.Observe(e.ctx)
		for {
			select {
			case node := <-observe:
				if string(node.Kvs[0].Value) == e.Candidate {
					log.Debug("IS Leader")
					atomic.StoreInt32(&e.isLeader, 1)
				} else {
					// We are not leader
					log.Debug("NOT Leader")
					atomic.StoreInt32(&e.isLeader, 0)
					return true
				}
			case <-e.session.Done():
				e.session.Close()
				goto sleep
			}
		}

	sleep:
		// Back off timeout
		tick := time.NewTicker(time.Second * 3)
		defer tick.Stop()
		select {
		case <-done:
			return false
		case <-e.ctx.Done():
			return false
		case <-tick.C:
		}

		log.Debug("Disconnected, creating new session")
		err = e.newSession()
		if err != nil {
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
	return err
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
		if err := e.election.Resign(e.ctx); err != nil {
			log.WithField("err", err).
				Error("while attempting to concede the election")
		}
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
