package election

import (
	"context"
	"sync"

	"github.com/mailgun/holster/v3/setter"
)

type ObsPair struct {
	From   string
	Leader string
}

// Useful in tests where you need to simulate an election cluster
type TestCluster struct {
	Nodes      map[string]Candidate
	ObserverCh chan ObsPair
	errors     map[string]error
	lock       sync.Mutex
}

func NewTestCluster() *TestCluster {
	return &TestCluster{
		Nodes:      make(map[string]Candidate),
		errors:     make(map[string]error),
		ObserverCh: make(chan ObsPair, 500),
	}
}

// Spawns a new node and adds it to the cluster
func (c *TestCluster) SpawnNode(name string, conf *Config) error {
	setter.SetDefault(&conf, &Config{})
	var err error

	conf.Self = name
	conf.SendRPC = func(ctx context.Context, peer string, req RPCRequest, resp *RPCResponse) error {
		return c.sendRPC(name, peer, req, resp)
	}
	conf.Observer = func(s string) {
		c.ObserverCh <- ObsPair{
			From:   name,
			Leader: s,
		}
	}
	c.Nodes[name], err = SpawnCandidate(*conf)

	// Build a list of all the peers
	var peers []string
	for k, _ := range c.Nodes {
		peers = append(peers, k)
	}

	// Update our list of known peers
	for _, v := range c.Nodes {
		v.SetPeers(peers)
	}

	return err
}

type ClusterStatus map[string]string

func (c *TestCluster) GetClusterStatus() ClusterStatus {
	status := make(ClusterStatus)
	for k, v := range c.Nodes {
		status[k] = v.Leader()
	}
	return status
}

func (c *TestCluster) GetLeader() Candidate {
	for _, v := range c.Nodes {
		if v.IsLeader() {
			return v
		}
	}
	return nil
}

func (c *TestCluster) AddNetworkError(peer string, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.errors[peer] = err
}

func (c *TestCluster) DelNetworkError(peer string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.errors, peer)
}

func (c *TestCluster) sendRPC(from string, to string, req RPCRequest, resp *RPCResponse) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err, ok := c.errors[from]; ok {
		return err
	}

	if err, ok := c.errors[to]; ok {
		return err
	}

	c.Nodes[to].ReceiveRPC(req, resp)
	return nil
}

func (c *TestCluster) Close() {
	for _, v := range c.Nodes {
		v.Close()
	}
}
