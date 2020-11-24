package election_test

import (
	"errors"
	"testing"
	"time"

	"github.com/mailgun/holster/v3/election"
	"github.com/mailgun/holster/v3/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	cfg            *election.Config
	ErrConnRefused = errors.New("connection refused")
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	cfg = &election.Config{
		NetworkTimeout:      time.Second,
		HeartBeatTimeout:    time.Second,
		LeaderQuorumTimeout: time.Second * 2,
		ElectionTimeout:     time.Second * 2,
	}
}

func TestSimpleElection(t *testing.T) {
	c := election.NewTestCluster()
	defer c.Close()

	// Ensure a single node becomes leader
	c.SpawnNode("n0", cfg)

	testutil.UntilPass(t, 30, time.Second, func(t testutil.TestingT) {
		status := c.GetClusterStatus()
		assert.Equal(t, election.ClusterStatus{
			"n0": "n0",
		}, status)
	})

	// Added nodes should become followers
	c.SpawnNode("n1", cfg)
	c.SpawnNode("n2", cfg)
	c.SpawnNode("n3", cfg)
	c.SpawnNode("n4", cfg)

	testutil.UntilPass(t, 30, time.Second, func(t testutil.TestingT) {
		status := c.GetClusterStatus()
		assert.Equal(t, election.ClusterStatus{
			"n0": "n0",
			"n1": "n0",
			"n2": "n0",
			"n3": "n0",
			"n4": "n0",
		}, status)
	})

	c.Nodes["n0"].Resign()

	// Wait until n0 is no longer leader
	testutil.UntilPass(t, 30, time.Second, func(t testutil.TestingT) {
		candidate := c.GetLeader()
		if !assert.NotNil(t, candidate) {
			return
		}
		assert.NotEqual(t, "n0", candidate.Leader())
	})

	for k, v := range c.Nodes {
		t.Logf("Node: %s Leader: %t\n", k, v.IsLeader())
	}
}

func TestLeaderDisconnect(t *testing.T) {
	c := election.NewTestCluster()
	defer c.Close()

	// Start with a known leader
	c.SpawnNode("n0", cfg)
	testutil.UntilPass(t, 10, time.Second, func(t testutil.TestingT) {
		status := c.GetClusterStatus()
		assert.Equal(t, election.ClusterStatus{
			"n0": "n0",
		}, status)
	})

	// Added nodes should become followers
	c.SpawnNode("n1", cfg)
	c.SpawnNode("n2", cfg)
	c.SpawnNode("n3", cfg)
	c.SpawnNode("n4", cfg)

	testutil.UntilPass(t, 10, time.Second, func(t testutil.TestingT) {
		status := c.GetClusterStatus()
		assert.Equal(t, election.ClusterStatus{
			"n0": "n0",
			"n1": "n0",
			"n2": "n0",
			"n3": "n0",
			"n4": "n0",
		}, status)
	})

	c.AddNetworkError("n0", ErrConnRefused)
	defer c.DelNetworkError("n0")

	// Should lose leadership
	testutil.UntilPass(t, 30, time.Second, func(t testutil.TestingT) {
		candidate := c.Nodes["n0"]
		if !assert.NotNil(t, candidate) {
			return
		}
		assert.NotEqual(t, "n0", candidate.Leader())
	})

	for k, v := range c.Nodes {
		t.Logf("Node: %s Leader: %t\n", k, v.IsLeader())
	}
}

// TODO: Test Follower disconnect (Should NOT force leader to
