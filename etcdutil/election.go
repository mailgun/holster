package etcdutil

import (
	"context"
	"os"
	"path"
	"sync"
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

	client   *etcd.Client
	concede  chan struct{}
	conceded sync.WaitGroup
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
//		// Do thing
//	}
func NewElection(election, candidate string, client *etcd.Client) (*Election, error) {
	log = logrus.WithField("category", "election")
	ctx, cancelFunc := context.WithCancel(context.Background())
	e := &Election{
		Candidate: candidate,
		Election:  election,
		TTL:       5,
		concede:   make(chan struct{}),
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

func (e *Election) Start() (err error) {
	var once sync.Once
	var completed sync.WaitGroup
	var election *concurrency.Election

	var concede = func() {
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(e.TTL)*time.Second)

		if err := election.Resign(ctx); err != nil {
			log.WithField("err", err).
				Error("while attempting to concede the election")
		}
		atomic.StoreInt32(&e.isLeader, 0)
		cancel()
	}

	completed.Add(1)
	e.wg.Until(func(done chan struct{}) bool {

		session, err := concurrency.NewSession(e.client, concurrency.WithTTL(e.TTL))
		if err != nil {
			err = errors.Wrap(err, "while creating new session")
			return false
		}

		election = concurrency.NewElection(session, e.Election)
		once.Do(completed.Done)

		// Start a new campaign and attempt to become leader
		if err = election.Campaign(e.ctx, e.Candidate); err != nil {
			err = errors.Wrap(err, "while starting a new campaign")
			return false
		}

		atomic.StoreInt32(&e.isLeader, 1)

		select {
		case <-session.Done():
			return true
		case <-e.concede:
			concede()
			e.conceded.Done()
			return true
		case <-done:
			concede()
			return false
		}
		return true
	})

	// Get the outcome of the election
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	completed.Wait()
	observeChan := election.Observe(ctx)
	select {
	case item := <-observeChan:
		if string(item.Kvs[0].Value) == e.Candidate {
			atomic.StoreInt32(&e.isLeader, 1)
		}
	}
	return err
}

func (e *Election) Stop() {
	e.cancel()
	e.wg.Stop()
}

func (e *Election) IsLeader() bool {
	return atomic.LoadInt32(&e.isLeader) == 1
}

// Release leadership and return true if we own it, else do nothing and return false
func (e *Election) Concede() bool {
	wasLeader := e.IsLeader()
	e.conceded.Add(1)
	e.concede <- struct{}{}
	e.conceded.Wait()
	return wasLeader
}

// Return the current leader of the election without joining the election
func (e *Election) LeaderPeek() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)

	session, err := concurrency.NewSession(e.client, concurrency.WithTTL(e.TTL))
	if err != nil {
		return "", errors.Wrap(err, "while creating new session")
	}
	election := concurrency.NewElection(session, e.Election)
	leader, err := election.Leader(ctx)
	cancel()
	if err != nil {
		return "", err
	}
	return string(leader.Kvs[0].Value), nil
}

type LeaderElectionMock struct{}

func (s *LeaderElectionMock) LeaderPeek() (string, error) { return "", nil }
func (s *LeaderElectionMock) IsLeader() bool              { return true }
func (s *LeaderElectionMock) Concede() bool               { return true }
func (s *LeaderElectionMock) Start() error                { return nil }
func (s *LeaderElectionMock) Stop()                       {}
