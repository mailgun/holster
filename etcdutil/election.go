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

	etcdConfig *etcd.Config
	session    *concurrency.Session
	election   *concurrency.Election
	client     *etcd.Client
	cancel     context.CancelFunc
	wg         holster.WaitGroup
	ctx        context.Context
	isLeader   int32
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
func NewElection(election, candidate string, etcdConfig *etcd.Config) (*Election, error) {
	log = logrus.WithField("category", "election")
	ctx, cancelFunc := context.WithCancel(context.Background())
	e := &Election{
		Candidate:  candidate,
		Election:   election,
		TTL:        5,
		etcdConfig: etcdConfig,
		cancel:     cancelFunc,
		ctx:        ctx,
	}

	if host, err := os.Hostname(); err == nil {
		holster.SetDefault(&e.Candidate, host)
	}

	// Set a prefix key for elections
	e.Election = path.Join("/elections", e.Election)

	var err error
	e.etcdConfig, err = NewEtcdConfig(etcdConfig)
	if err != nil {
		return nil, err
	}

	e.client, err = etcd.New(*e.etcdConfig)
	if err != nil {
		return nil, err
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	_, err = e.client.Get(ctx, e.Election)
	if err != nil {
		return nil, errors.Wrap(err, "while connecting to etcd")
	}

	return e, nil
}

func (e *Election) Start() error {
	var err error

	e.session, err = concurrency.NewSession(e.client, concurrency.WithTTL(e.TTL))
	if err != nil {
		return errors.Wrap(err, "while creating new session")
	}

	// Start a new election
	e.election = concurrency.NewElection(e.session, e.Election)

	e.wg.Until(func(done chan struct{}) bool {
		log.Debugf("attempting to become leader '%s'\n", e.Candidate)

		// Start a new campaign and attempt to become leader
		if err := e.election.Campaign(e.ctx, e.Candidate); err != nil {
			errors.Wrap(err, "while starting a new campaign")
		}

		observeChan := e.election.Observe(e.ctx)
		for {
			select {
			case node, ok := <-observeChan:
				if !ok {
					return false
				}
				if string(node.Kvs[0].Value) == e.Candidate {
					log.Debug("IS Leader")
					atomic.StoreInt32(&e.isLeader, 1)
				} else {
					// We are not leader
					logrus.Debug("NOT Leader")
					atomic.StoreInt32(&e.isLeader, 0)
				}
			case <-done:
				return false
			}
		}
	})
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
		if err := e.election.Resign(e.ctx); err != nil {
			logrus.WithField("err", err).
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
func (s *LeaderElectionMock) Start()         {}
func (s *LeaderElectionMock) Stop()          {}
