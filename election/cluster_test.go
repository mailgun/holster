package election_test

import (
	"context"
	"fmt"
	"sync"

	"github.com/mailgun/holster/v3/election"
	"github.com/mailgun/holster/v3/setter"
)

type ChangePair struct {
	From   string
	Leader string
}

// Useful in tests where you need to simulate an election cluster
type TestCluster struct {
	Nodes      map[string]*ClusterNode
	OnChangeCh chan ChangePair
	errors     map[string]error
	lock       sync.Mutex
}

type ClusterNode struct {
	lock    sync.RWMutex
	Node    election.Node
	SendRPC func(from string, to string, req election.RPCRequest, resp *election.RPCResponse) error
}

func NewTestCluster() *TestCluster {
	return &TestCluster{
		Nodes:      make(map[string]*ClusterNode),
		errors:     make(map[string]error),
		OnChangeCh: make(chan ChangePair, 500),
	}
}

// Spawns a new node and adds it to the cluster
func (c *TestCluster) SpawnNode(name string, conf *election.Config) error {
	setter.SetDefault(&conf, &election.Config{})
	n := &ClusterNode{
		SendRPC: c.sendRPC,
	}

	conf.UniqueID = name
	conf.SendRPC = func(ctx context.Context, peer string, req election.RPCRequest, resp *election.RPCResponse) error {
		n.lock.RLock()
		defer n.lock.RUnlock()
		return n.SendRPC(name, peer, req, resp)
	}
	conf.OnUpdate = func(s string) {
		c.OnChangeCh <- ChangePair{
			From:   name,
			Leader: s,
		}
	}
	var err error
	n.Node, err = election.NewNode(*conf)
	if err != nil {
		return err
	}
	// Add the node to our list of nodes
	c.Add(name, n)
	n.Node.Start()
	return nil
}

func (c *TestCluster) Add(name string, node *ClusterNode) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.Nodes[name] = node

	node.lock.Lock()
	defer node.lock.Unlock()
	node.SendRPC = c.sendRPC
	c.updatePeers()
}

func (c *TestCluster) Remove(name string) *ClusterNode {
	n := c.Nodes[name]
	delete(c.Nodes, name)
	c.updatePeers()
	return n
}

func (c *TestCluster) updatePeers() {
	// Build a list of all the peers
	var peers []string
	for k, _ := range c.Nodes {
		peers = append(peers, k)
	}

	// Update our list of known peers
	for _, v := range c.Nodes {
		v.Node.SetPeers(peers)
	}
}

type ClusterStatus map[string]string

func (c *TestCluster) GetClusterStatus() ClusterStatus {
	status := make(ClusterStatus)
	for k, v := range c.Nodes {
		status[k] = v.Node.GetLeader()
	}
	return status
}

func (c *TestCluster) GetLeader() election.Node {
	for _, v := range c.Nodes {
		if v.Node.IsLeader() {
			return v.Node
		}
	}
	return nil
}

func (c *TestCluster) peerKey(from, to string) string {
	return fmt.Sprintf("%s|%s", from, to)
}

func (c *TestCluster) ClearErrors() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.errors = make(map[string]error)
}

// Add a specific peer to peer error
func (c *TestCluster) AddPeerToPeerError(from string, to string, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.errors[c.peerKey(from, to)] = err
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

func (c *TestCluster) sendRPC(from string, to string, req election.RPCRequest, resp *election.RPCResponse) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err, ok := c.errors[from]; ok {
		return err
	}

	if err, ok := c.errors[to]; ok {
		return err
	}

	if err, ok := c.errors[c.peerKey(from, to)]; ok {
		return err
	}

	n, ok := c.Nodes[to]
	if !ok {
		return fmt.Errorf("unknown peer '%s'", to)
	}
	n.Node.ReceiveRPC(req, resp)

	return nil
}

func (c *TestCluster) Close() {
	for _, v := range c.Nodes {
		v.Node.Stop()
	}
}
