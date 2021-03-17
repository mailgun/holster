package election_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mailgun/holster/v3/election"
	"github.com/mailgun/holster/v3/slice"
	"github.com/mailgun/holster/v3/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func createCluster(t *testing.T, c *TestCluster) {
	t.Helper()

	// Start with a known leader
	err := c.SpawnNode("n0", cfg)
	require.NoError(t, err)
	testutil.UntilPass(t, 10, time.Second, func(t testutil.TestingT) {
		status := c.GetClusterStatus()
		assert.Equal(t, ClusterStatus{
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
		assert.Equal(t, ClusterStatus{
			"n0": "n0",
			"n1": "n0",
			"n2": "n0",
			"n3": "n0",
			"n4": "n0",
		}, status)
	})
}

func TestSingleNodeLeader(t *testing.T) {
	c := NewTestCluster()
	err := c.SpawnNode("n0", cfg)
	require.NoError(t, err)
	testutil.UntilPass(t, 10, time.Second, func(t testutil.TestingT) {
		status := c.GetClusterStatus()
		assert.Equal(t, ClusterStatus{
			"n0": "n0",
		}, status)
	})

	// Consume first leader election event
	event := <-c.OnChangeCh
	assert.Equal(t, "n0", event.Leader)
	assert.Equal(t, "n0", event.From)

	assert.True(t, c.Nodes["n0"].Node.IsLeader())

	select {
	// Should NOT receive a leadership change as we are the only node
	case <-c.OnChangeCh:
		t.Log("received un-expected leader change")
		t.FailNow()
	case <-time.After(cfg.HeartBeatTimeout * 3):
	}
}

func TestSimpleElection(t *testing.T) {
	c := NewTestCluster()
	createCluster(t, c)
	defer c.Close()

	c.Nodes["n0"].Node.Resign(context.Background())

	// Wait until n0 is no longer leader
	testutil.UntilPass(t, 30, time.Second, func(t testutil.TestingT) {
		candidate := c.GetLeader()
		if !assert.NotNil(t, candidate) {
			return
		}
		assert.NotEqual(t, "n0", candidate.GetLeader())
	})

	for k, v := range c.Nodes {
		t.Logf("Node: %s Leader: %t\n", k, v.Node.IsLeader())
	}
}

func TestLeaderDisconnect(t *testing.T) {
	c := NewTestCluster()
	createCluster(t, c)
	defer c.Close()

	c.AddNetworkError("n0", ErrConnRefused)
	defer c.DelNetworkError("n0")

	// Should lose leadership
	testutil.UntilPass(t, 30, time.Second, func(t testutil.TestingT) {
		node := c.Nodes["n0"]
		if !assert.NotNil(t, node.Node) {
			return
		}
		assert.NotEqual(t, "n0", node.Node.GetLeader())
	})

	for k, v := range c.Nodes {
		t.Logf("Node: %s Leader: %t\n", k, v.Node.IsLeader())
	}
}

func TestFollowerDisconnect(t *testing.T) {
	c := NewTestCluster()
	createCluster(t, c)
	defer c.Close()

	c.AddNetworkError("n4", ErrConnRefused)
	defer c.DelNetworkError("n4")

	// Wait until n4 loses leader
	testutil.UntilPass(t, 5, time.Second, func(t testutil.TestingT) {
		status := c.GetClusterStatus()
		assert.NotEqual(t, "n0", status["n4"])
	})

	c.DelNetworkError("n4")

	// Follower should resume being a follower without forcing a new election.
	testutil.UntilPass(t, 60, time.Second, func(t testutil.TestingT) {
		status := c.GetClusterStatus()
		assert.Equal(t, "n0", status["n4"])
	})
}

func TestSplitBrain(t *testing.T) {
	c1 := NewTestCluster()
	createCluster(t, c1)
	defer c1.Close()

	c2 := NewTestCluster()

	// Now take 2 nodes from cluster 1 and put them in their own cluster.
	// This causes n0 to lose contact with n2-n4 and should update the member list
	// such that n0 only knows about n1.

	// Since n0 was leader previously, it should remain leader
	c2.Add("n0", c1.Remove("n0"))
	c2.Add("n1", c1.Remove("n1"))

	// Cluster 1 should elect a new leader
	testutil.UntilPass(t, 30, time.Second, func(t testutil.TestingT) {
		assert.NotNil(t, c1.GetLeader())
	})

	for k, v := range c1.Nodes {
		t.Logf("C1 Node: %s Leader: %t\n", k, v.Node.IsLeader())
	}

	// Cluster 2 should elect a new leader
	testutil.UntilPass(t, 30, time.Second, func(t testutil.TestingT) {
		assert.NotNil(t, c2.GetLeader())
	})

	for k, v := range c2.Nodes {
		t.Logf("C2 Node: %s Leader: %t\n", k, v.Node.IsLeader())
	}

	// Move the nodes in cluster2, back to the cluster1
	c1.Add("n0", c2.Remove("n0"))
	c1.Add("n1", c2.Remove("n1"))

	// The nodes should detect 2 leaders and start a new vote.
	testutil.UntilPass(t, 10, time.Second, func(t testutil.TestingT) {
		status := c1.GetClusterStatus()
		var leaders []string
		for _, v := range status {
			if slice.ContainsString(v, leaders, nil) {
				continue
			}
			leaders = append(leaders, v)
		}
		if !assert.NotNil(t, leaders) {
			return
		}
		assert.Equal(t, 1, len(leaders))
		assert.NotEmpty(t, leaders[0])
	})

	for k, v := range c1.Nodes {
		t.Logf("Node: %s Leader: %t\n", k, v.Node.IsLeader())
	}
}

func TestOmissionFaults(t *testing.T) {
	c1 := NewTestCluster()
	createCluster(t, c1)
	defer c1.Close()

	// Create an unstable cluster with n3 and n4 only able to contact n1 and n2 respectively.
	// The end result should be an omission fault of less than quorum.
	//
	// Diagram: lines indicate connectivity between nodes
	// (n0)-----(n1)----(n4)
	//   \       /
	//	  \     /
	//     \   /
	//      (n2)----(n3)
	//

	// n3 and n4 can't talk
	c1.Disconnect("n3", "n4", ErrConnRefused)
	c1.Disconnect("n4", "n3", ErrConnRefused)

	// Leader can't talk to n4
	c1.Disconnect("n0", "n4", ErrConnRefused)
	c1.Disconnect("n4", "n0", ErrConnRefused)

	// Leader can't talk to n3
	c1.Disconnect("n0", "n3", ErrConnRefused)
	c1.Disconnect("n3", "n0", ErrConnRefused)

	// n2 and n4 can't talk
	c1.Disconnect("n2", "n4", ErrConnRefused)
	c1.Disconnect("n4", "n2", ErrConnRefused)

	// n1 and n3 can't talk
	c1.Disconnect("n1", "n3", ErrConnRefused)
	c1.Disconnect("n3", "n1", ErrConnRefused)

	// Cluster should retain n0 as leader in the face on unstable cluster
	for i := 0; i < 12; i++ {
		leader := c1.GetLeader()
		require.NotNil(t, leader)
		require.Equal(t, leader.GetLeader(), "n0")
		time.Sleep(time.Millisecond * 400)
	}

	// Should retain leader once communication is restored
	c1.ClearErrors()

	for i := 0; i < 12; i++ {
		leader := c1.GetLeader()
		require.NotNil(t, leader)
		require.Equal(t, leader.GetLeader(), "n0")
		time.Sleep(time.Millisecond * 400)
	}
}

func TestIsolatedLeader(t *testing.T) {
	c1 := NewTestCluster()
	createCluster(t, c1)
	defer c1.Close()

	// Create a cluster where the leader become isolated from the rest
	// of the cluster.
	//
	// Diagram: lines indicate connectivity
	// between nodes and n0 is leader
	//
	// (n0)----(n1)----(n4)
	//          / \     /
	//	       /   \   /
	//        /     \ /
	//      (n2)----(n3)
	//
	require.Equal(t, c1.GetLeader().GetLeader(), "n0")

	// Leader can't talk to n2
	c1.Disconnect("n0", "n2", ErrConnRefused)
	c1.Disconnect("n2", "n0", ErrConnRefused)

	// Leader can't talk to n3
	c1.Disconnect("n0", "n3", ErrConnRefused)
	c1.Disconnect("n3", "n0", ErrConnRefused)

	// Leader can't talk to n4
	c1.Disconnect("n0", "n4", ErrConnRefused)
	c1.Disconnect("n4", "n0", ErrConnRefused)

	// Leader should realize it doesn't have a quorum of
	// heartbeats and step down and remaining cluster should
	// elect a new leader
	for i := 0; i < 20; i++ {
		leader := c1.GetLeader()
		if leader == nil {
			goto sleep
		}

		// Leader should no longer be n0
		if leader.GetLeader() != "n0" {
			// A node in the new cluster must agree and have elected a new leader
			l := c1.Nodes["n4"].Node.GetLeader()
			if l != "" && l == "n0" {
				break
			}
		}
	sleep:
		time.Sleep(time.Millisecond * 500)
	}
	require.NotNil(t, c1.GetLeader())
	require.NotEqual(t, c1.GetLeader().GetLeader(), "n0")
	// Note: In the case where n1 is elected the new leader,
	// n0 will know that n1 is the new leader sooner than later
	// since connectivity from n0 to n1 was never interrupted.
	//fmt.Printf("Cluster: %#v\n", c1.GetClusterStatus())

	// Should persist new leader once communication is restored
	c1.ClearErrors()

	//for i := 0; i < 20; i++ {
	//	if c1.Nodes["n0"].Node.GetLeader() == "" {
	//		time.Sleep(time.Millisecond * 500)
	//		continue
	//	}
	//	break
	//}

	// Should pick up the leadership from the rest of the cluster
	testutil.UntilPass(t, 10, time.Second, func(t testutil.TestingT) {
		leader := c1.Nodes["n0"].Node.GetLeader()
		assert.NotEqual(t, leader, "")
	})

	s, err := c1.Nodes["n0"].Node.GetState(context.Background())
	fmt.Printf("State: %#v\n", s)
	require.NoError(t, err)
	assert.Equal(t, "Follower", s.State)
}

func TestMinimumQuorum(t *testing.T) {
	c := NewTestCluster()

	cfg := &election.Config{
		NetworkTimeout:      time.Second,
		HeartBeatTimeout:    time.Second,
		LeaderQuorumTimeout: time.Second * 2,
		ElectionTimeout:     time.Second * 2,
		MinimumQuorum:       2,
	}

	err := c.SpawnNode("n0", cfg)
	require.NoError(t, err)

	time.Sleep(time.Second * 5)

	// Ensure n0 is not leader
	status := c.GetClusterStatus()
	require.NotEqual(t, "n0", status["n0"])

	c.SpawnNode("n1", cfg)

	// Should elect a leader
	testutil.UntilPass(t, 10, time.Second, func(t testutil.TestingT) {
		status := c.GetClusterStatus()
		assert.NotEqual(t, status["n0"], "")
	})

	status = c.GetClusterStatus()
	var leader string

	// Shutdown the follower
	if status["n0"] == "n0" {
		c.Remove("n1").Node.Stop(context.Background())
		leader = "n0"
	} else {
		c.Remove("n0").Node.Stop(context.Background())
		leader = "n1"
	}

	// The leader should detect it no longer has MinimumQuorum and step down
	testutil.UntilPass(t, 10, time.Second, func(t testutil.TestingT) {
		status := c.GetClusterStatus()
		assert.Equal(t, status[leader], "")
	})
}
