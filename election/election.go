package election

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync/atomic"
	"time"

	"github.com/mailgun/holster/v4/setter"
	"github.com/mailgun/holster/v4/slice"
	"github.com/mailgun/holster/v4/syncutil"
	"github.com/sirupsen/logrus"
)

type NodeState GetStateResp
type state uint32

const (
	// followerState means we are following the leader and expect
	// to get heart beats regularly. This is the initial state, as
	// we don't want to force an election when a new node joins
	// the cluster.
	followerState state = iota
	// candidateState means we are actively attempting to become leader
	candidateState
	// leaderState means we have received a quorum of votes while
	// in candidateState and have assumed leadership.
	leaderState
	// shutdownState means we are in the process of shutting down
	shutdownState
)

var ErrNotLeader = errors.New("not the leader")

func (s state) String() string {
	switch s {
	case followerState:
		return "Follower"
	case candidateState:
		return "Candidate"
	case leaderState:
		return "Leader"
	case shutdownState:
		return "Shutdown"
	default:
		return "Unknown"
	}
}

type SendRPCFunc func(context.Context, string, RPCRequest, *RPCResponse) error

type Config struct {
	// How long we should wait for a single network operation to complete.
	NetworkTimeout time.Duration

	// How long followers should wait before they decide the leader
	// lost connection to peers and therefore start a new election.
	HeartBeatTimeout time.Duration

	// How long candidates should wait for an election to complete
	// before starting a new one.
	ElectionTimeout time.Duration

	// How long the leader should wait on heart beat responses from
	// followers before it decides to step down as leader and start a
	// new election.
	LeaderQuorumTimeout time.Duration

	// The minimum number of peers that are required to form a cluster and elect a leader.
	// This is to prevent a small number of nodes (or a single node) that gets disconnected
	// from the cluster to elect a leader (assuming the peer list is updated to exclude the
	// disconnected peers). Instead nodes will wait until connectivity is restored
	// to the quorum of the cluster. The default is zero, which means if a single node is
	// disconnected from the cluster, and it's peer list only includes it's self, it will elect
	// itself leader. If we set MinimumQuorum = 2 then no leader will be elected until the peer
	// list includes at least 2 peers and a successful vote has completed.
	MinimumQuorum int

	// The Initial list of peers to be considered in the election, including ourself.
	Peers []string

	// The unique id this peer identifies itself as, as found in the Peers list.
	// This is typically an ip:port address the node is listening to for RPC requests.
	UniqueID string

	// Called when the leader changes
	OnUpdate OnUpdate

	// The logger used errors and warning
	Log logrus.FieldLogger

	// Sends an RPC request to a peer, This function must be provided and can
	// utilize any network communication the implementer wishes. If context cancelled
	// should return an error.
	SendRPC SendRPCFunc
}

type OnUpdate func(string)

type Node interface {
	// Starts the main election loop.
	Start(ctx context.Context) error

	// Cancels the election, resigns if we are leader and waits for all go
	// routines to complete before returning.
	Stop(ctx context.Context) error

	// Set the list of peers to be considered for the election
	SetPeers(ctx context.Context, peers []string) error

	// If leader, resigns as leader and starts a new election that we will not
	// participate in. returns ErrNotLeader if not currently the leader
	Resign(ctx context.Context) error

	// IsLeader is a convenience function that calls GetState() and returns true
	// if this node was elected leader. May block if main loop is occupied.
	IsLeader() bool

	// GetLeader is a convenience function that calls GetState() returns the
	// unique id of the node that is currently leader. May block if main loop is occupied.
	GetLeader() string

	// Returns the current state of this node
	GetState(ctx context.Context) (NodeState, error)

	// Called when this peer receives a RPC request from a peer
	ReceiveRPC(RPCRequest, *RPCResponse)
}

type node struct {
	conf  Config // The election configuration
	state state  // Current state of our node
	vote  struct {
		CurrentTerm   uint64
		LastTerm      uint64
		LastCandidate string
	} // Current state of the vote
	currentTerm uint64          // The current term of the election when in candidate state
	rpcCh       chan RPCRequest // RPC Response channel, listen for for RPC responses on this channel
	self        string          // Our name
	peers       []string
	leader      string
	lastContact time.Time     // The last successful contact with the leader (if we are a follower)
	shutdownCh  chan struct{} // Signals we are in shutdown
	log         logrus.FieldLogger
	wg          syncutil.WaitGroup
	running     int64
}

// Creates a new node. You must call Start() to be participate in the election.
func NewNode(conf Config) (Node, error) {
	if conf.UniqueID == "" {
		return nil, errors.New("refusing to spawn a new node with no Config.UniqueID defined")
	}

	if conf.SendRPC == nil {
		return nil, errors.New("refusing to spawn a new node with no Config.SendRPC defined")
	}

	setter.SetDefault(&conf.Log, logrus.WithField("id", conf.UniqueID))
	setter.SetDefault(&conf.LeaderQuorumTimeout, time.Second*12)
	setter.SetDefault(&conf.HeartBeatTimeout, time.Second*6)
	setter.SetDefault(&conf.ElectionTimeout, time.Second*6)
	setter.SetDefault(&conf.NetworkTimeout, time.Second*3)

	c := &node{
		rpcCh: make(chan RPCRequest, 5_000),
		self:  conf.UniqueID,
		peers: conf.Peers,
		conf:  conf,
		log:   conf.Log,
	}
	return c, nil
}

// Called by the implementer when an RPC is received from another node
func (e *node) ReceiveRPC(req RPCRequest, resp *RPCResponse) {
	// Ignore requests received when we are not running. If
	// we don't we can create a race when initializing e.shutdownCh
	// and we could fill up the rpcCh with requests that are never handled
	if atomic.LoadInt64(&e.running) != 1 {
		return
	}

	req.respChan = make(chan RPCResponse, 1)
	e.rpcCh <- req

	select {
	case rpcResp := <-req.respChan:
		*resp = rpcResp
	case <-e.shutdownCh:
	}
}

// SetPeers is a thread safe way to dynamically add or remove peers in a running cluster.
// These peers will be contacted when requesting votes during leader election.
func (e *node) SetPeers(ctx context.Context, peers []string) error {

	// If the main loop is not running, there is no risk of race
	if atomic.LoadInt64(&e.running) != 1 {
		e.peers = peers
		return nil
	}

	select {
	case <-e.send(SetPeersReq{Peers: peers}):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetState returns the current state of this node
func (e *node) GetState(ctx context.Context) (NodeState, error) {

	// If the main loop is not running, there is no risk of race
	if atomic.LoadInt64(&e.running) != 1 {
		return NodeState{
			Peers:  e.peers,
			State:  e.state.String(),
			Leader: e.leader,
		}, nil
	}

	select {
	case resp := <-e.send(GetStateReq{}):
		if s, ok := resp.Response.(GetStateResp); ok {
			return NodeState(s), nil
		}
	case <-ctx.Done():
		return NodeState{}, ctx.Err()
	}
	return NodeState{}, nil
}

// IsLeader is a convenience function that calls GetState() and returns true
// if this node was elected leader. May block if main loop is occupied.
func (e *node) IsLeader() bool {
	s, _ := e.GetState(context.Background())
	return e.self == s.Leader
}

// GetLeader is a convenience function that calls GetState() returns the
// unique id of the node that is currently leader. May block if main loop is occupied.
func (e *node) GetLeader() string {
	s, _ := e.GetState(context.Background())
	return s.Leader
}

func (e *node) isLeader() bool {
	return e.self == e.leader
}

func (e *node) setLeader(leader string) {
	if e.leader != leader {
		e.log.Debugf("Set Leader '%s'", leader)
		e.leader = leader
		if e.conf.OnUpdate != nil {
			e.conf.OnUpdate(leader)
		}
	}
}

func (e *node) getLeader() string {
	return e.leader
}

// Resign will cause this node to step down as leader, if this
// node is NOT leader, this does nothing and returns ErrNotLeader
func (e *node) Resign(ctx context.Context) error {

	// Avoid blocking if main loop is not running
	if atomic.LoadInt64(&e.running) != 1 {
		return ErrNotLeader
	}

	select {
	case rpcResp := <-e.send(ResignReq{}):
		resp, ok := rpcResp.Response.(ResignResp)
		if !ok {
			return errors.New("resign response channel closed")
		}
		if rpcResp.Error != "" {
			return errors.New(rpcResp.Error)
		}
		if resp.Success {
			return nil
		}
		return ErrNotLeader
	case <-e.shutdownCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Start the main event loop which allows the election to proceed.
// Call this method when the node is ready to be considered in the election.
func (e *node) Start(ctx context.Context) error {
	if atomic.LoadInt64(&e.running) == 1 {
		return nil
	}
	e.shutdownCh = make(chan struct{})
	atomic.StoreInt64(&e.running, 1)
	e.wg.Go(e.run)
	return nil
}

// Stop stops all internal go routines and if this node is currently
// leader, resigns as leader.
func (e *node) Stop(ctx context.Context) error {
	if atomic.LoadInt64(&e.running) != 1 {
		return nil
	}
	atomic.StoreInt64(&e.running, 0)
	close(e.shutdownCh)
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return ctx.Err()
	}
}

// Main thread loop
func (e *node) run() {
	for {
		select {
		case <-e.shutdownCh:
			e.state = shutdownState
			return
		default:
		}

		switch e.state {
		case followerState:
			e.runFollower()
		case candidateState:
			e.runCandidate()
		case leaderState:
			e.runLeader()
		}
	}
}

func (e *node) runFollower() {
	e.log.Debugf("entering follower state, current leader is '%s'", e.leader)
	timeout := randomDuration(e.conf.HeartBeatTimeout)
	heartbeatTimer := time.NewTicker(timeout)
	defer heartbeatTimer.Stop()
	noPeersTimer := time.NewTimer(timeout / 5)
	defer noPeersTimer.Stop()

	for e.state == followerState {
		select {
		case rpc := <-e.rpcCh:
			e.processRPC(rpc)
		case <-heartbeatTimer.C:
			// Check if we have had successful contact with the leader
			if time.Now().Sub(e.lastContact) < e.conf.HeartBeatTimeout {
				continue
			}

			// Heartbeat failed! Transition to the candidate state
			e.log.Debugf("heartbeat timeout, starting election; previous leader was '%s'", e.leader)
			e.setLeader("")
			e.state = candidateState
			return
		case <-noPeersTimer.C:
			// If we already have leader, don't check for no peers
			if e.getLeader() != "" {
				continue
			}

			// If we have no peers, or if we are the only peer, no need to wait
			// for the heartbeat timeout. Change state to candidate and start the election.
			if len(e.peers) == 0 || len(e.peers) == 1 && e.peers[0] == e.self {
				e.state = candidateState
				return
			}
		case <-e.shutdownCh:
			return
		}
	}
}

func (e *node) runCandidate() {
	e.log.Debugf("entering candidate state; current term '%d'", e.currentTerm+1)
	voteCh := make(<-chan VoteResp)

	// Each node will choose a random time to send their vote. This makes it more
	// likely that the first node to send vote requests will win the election, and avoid
	// a stalemate.
	voteTimer := time.NewTimer(randomDuration(e.conf.HeartBeatTimeout / 10))
	defer voteTimer.Stop()
	// We re-start the vote if we have not received a heart beat from a chosen leader before
	// this timer expires.
	electionTimer := time.NewTimer(randomDuration(e.conf.ElectionTimeout))
	defer electionTimer.Stop()

	// Tally the votes, need a simple majority
	grantedVotes := 0
	votesNeeded := e.quorumSize()
	e.log.Debugf("votes needed: %d", votesNeeded)

	for e.state == candidateState {
		select {
		case <-voteTimer.C:
			// Do not start a vote if we are below our minimum quorum
			if len(e.peers) < e.conf.MinimumQuorum {
				e.log.Warnf("peer count '%d' below minimum quorum of '%d'; sleeping...",
					len(e.peers), e.conf.MinimumQuorum)
				continue
			}
			voteCh = e.electSelf()
		case rpc := <-e.rpcCh:
			e.processRPC(rpc)
		case vote := <-voteCh:
			// Check if the term is greater than ours, bail
			if vote.Term > e.currentTerm {
				e.log.Debug("newer term discovered, fallback to follower")
				e.state = followerState
				e.currentTerm = vote.Term
				return
			}

			// Check if the vote is granted
			if vote.Granted {
				grantedVotes++
				e.log.Debugf("vote granted from '%s' term '%d', tally '%d'", vote.Candidate, vote.Term, grantedVotes)
			}

			// Check if we've become the leader
			if grantedVotes >= votesNeeded {
				e.log.Debugf("election won! tally is '%d'", grantedVotes)
				e.state = leaderState
				e.setLeader(e.self)
				return
			}
		case <-electionTimer.C:
			// Election failed! Restart the election. We simply return, which will kick us back into runCandidate
			e.log.Debug("Election timeout reached, restarting election")
			return
		case <-e.shutdownCh:
			return
		}
	}
}

// electSelf is used to send a VoteReq RPC to all peers with a vote for
// ourself. This has the side affect of incrementing the current term. The
// response channel returned is used to wait for all the responses, including a
// vote for ourself.
func (e *node) electSelf() <-chan VoteResp {
	respCh := make(chan VoteResp, len(e.peers)+1)

	// Increment the term
	e.currentTerm++

	// Construct a function to ask for a vote
	askPeer := func(peer string, term uint64, self string) {
		e.wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), e.conf.NetworkTimeout)
			defer cancel()

			// Construct the request
			req := RPCRequest{
				RPC: VoteRPC,
				Request: VoteReq{
					Term:      term,
					Candidate: self,
				},
			}

			var resp RPCResponse
			if err := e.conf.SendRPC(ctx, peer, req, &resp); err != nil {
				e.log.WithFields(logrus.Fields{"err": err, "peer": peer}).
					Error("error during vote rpc")
				vResp, ok := resp.Response.(VoteResp)
				if !ok {
					return
				}
				vResp.Term = term
				vResp.Granted = false
				respCh <- vResp
			}
			vResp, ok := resp.Response.(VoteResp)
			if !ok {
				return
			}
			respCh <- vResp
		})
	}

	// Vote for ourselves first
	e.vote.LastCandidate = e.self
	e.vote.LastTerm = e.currentTerm

	// Include our own vote
	respCh <- VoteResp{
		Candidate: e.self,
		Term:      e.currentTerm,
		Granted:   true,
	}

	// For each peer, request a vote
	for _, peer := range e.peers {
		if peer == e.self {
			continue
		}
		askPeer(peer, e.currentTerm, e.self)
	}
	return respCh
}

func (e *node) runLeader() {
	heartBeatTicker := time.NewTicker(randomDuration(e.conf.HeartBeatTimeout / 3))
	defer heartBeatTicker.Stop()
	quorumTicker := time.NewTicker(e.conf.LeaderQuorumTimeout)
	defer quorumTicker.Stop()
	peersLastContact := make(map[string]time.Time, len(e.peers))
	heartBeatReplyCh := make(chan HeartBeatResp, 5_000)

	for e.state == leaderState {
		select {
		case rpc := <-e.rpcCh:
			e.processRPC(rpc)
			// If the RPC was a set peers request, immediately send heart beats to all nodes
			if _, ok := rpc.Request.(SetPeersReq); ok {
				for _, peer := range e.peers {
					e.sendHeartBeat(peer, heartBeatReplyCh)
				}
			}
		case reply := <-heartBeatReplyCh:
			// Is the reply from a peer we are familiar with?
			if !slice.ContainsString(reply.From, e.peers, nil) {
				e.log.WithField("peer", reply.From).
					Debug("leader received heartbeat reply from peer not in our peer list; ignoring")
				break
			}
			peersLastContact[reply.From] = time.Now()
		case <-heartBeatTicker.C:
			for _, peer := range e.peers {
				e.sendHeartBeat(peer, heartBeatReplyCh)
			}
		case <-quorumTicker.C:
			// If the number of peers falls below our MinimumQuorum then we step down.
			if len(e.peers) < e.conf.MinimumQuorum {
				e.log.Warnf("peer count '%d' below minimum quorum of '%d'; stepping down",
					len(e.peers), e.conf.MinimumQuorum)
				e.state = followerState
				e.setLeader("")
				// Inform the other peers we are stepping down
				for _, peer := range e.peers {
					e.sendElectionReset(peer)
				}
				return
			}

			// Check if we have received contact from a quorum of nodes within the leader quorum timeout interval.
			// If not, we step down as we may have lost connectivity.
			contacted := 0
			now := time.Now()
			for _, peer := range e.peers {
				if peer == e.self {
					continue
				}

				lc, ok := peersLastContact[peer]
				if !ok {
					e.log.Debugf("quorum check - peer '%s' not found", peer)
					continue
				}
				diff := now.Sub(lc)
				e.log.Debugf("quorum check - peer '%s' diff '%f", peer, diff.Seconds())
				if diff >= e.conf.HeartBeatTimeout {
					e.log.Debugf("no heartbeat response from '%s' for '%s'", peer, diff)
					continue
				}
				contacted++
			}

			// Verify we can contact a quorum (Minus ourself)
			quorum := e.quorumSize()
			e.log.Debugf("quorum check - quorum='%d' contacted='%d'", quorum-1, contacted)
			if contacted < (quorum - 1) {
				e.log.Debug("failed to receive heart beats from a quorum of peers; stepping down")
				e.state = followerState
				e.setLeader("")
				// Inform the other peers we are stepping down
				for _, peer := range e.peers {
					e.sendElectionReset(peer)
				}
				return
			}
		case <-e.shutdownCh:
			e.state = shutdownState
			e.log.Debug("leader shutdown")
			if e.isLeader() {
				// Notify all followers we are no longer leader
				for _, peer := range e.peers {
					e.sendElectionReset(peer)
				}
			}
			return
		}
	}
	e.lastContact = time.Now()
	if e.isLeader() {
		e.setLeader("")
	}
}

func (e *node) sendHeartBeat(peer string, heartBeatReplyCh chan HeartBeatResp) {
	// Don't heartbeat ourself
	if peer == e.self {
		return
	}
	// Avoid race by localizing the current term
	term := e.currentTerm

	e.wg.Go(func() {
		var resp RPCResponse
		req := RPCRequest{
			RPC: HeartBeatRPC,
			Request: HeartBeatReq{
				From: e.self,
				Term: term,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), e.conf.NetworkTimeout)
		defer cancel()
		if err := e.conf.SendRPC(ctx, peer, req, &resp); err != nil {
			e.log.WithFields(logrus.Fields{"err": err, "peer": peer}).
				Debug("error during heart beat rpc")
			return
		}
		hResp, ok := resp.Response.(HeartBeatResp)
		if !ok {
			return
		}
		heartBeatReplyCh <- hResp
	})
}

func (e *node) sendElectionReset(peer string) {
	// Don't send election reset to ourself
	if peer == e.self {
		return
	}

	e.wg.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), e.conf.NetworkTimeout)
		defer cancel()
		req := RPCRequest{RPC: ResetElectionRPC, Request: ResetElectionReq{}}
		if err := e.conf.SendRPC(ctx, peer, req, &RPCResponse{}); err != nil {
			e.log.WithFields(logrus.Fields{"err": err, "peer": peer}).
				Debug("error during reset election rpc")
		}
	})
}

func (e *node) processRPC(rpc RPCRequest) {
	switch cmd := rpc.Request.(type) {
	case VoteReq:
		e.handleVote(rpc, cmd)
	case ResetElectionReq:
		e.handleResetElection(rpc)
	case HeartBeatReq:
		e.handleHeartBeat(rpc, cmd)
	case ResignReq:
		e.handleResign(rpc)
	case SetPeersReq:
		e.handleSetPeers(rpc, cmd)
	case GetStateReq:
		e.handleGetState(rpc)
	default:
		e.log.Errorf("got unexpected command %#v", rpc.Request)
		rpc.respond(rpc.RPC, nil, "unexpected command")
	}
}

// handleResign Notifies all followers that we are stepping down as leader.
// if we are leader returns Success = true
func (e *node) handleResign(rpc RPCRequest) {
	e.log.Debug("RPC: election.ResignReq{}")
	// If not leader, do nothing
	if !e.isLeader() {
		rpc.respond(rpc.RPC, ResignResp{}, "")
		return
	}

	e.setLeader("")
	e.state = followerState
	for _, peer := range e.peers {
		e.sendElectionReset(peer)
	}
	rpc.respond(rpc.RPC, ResignResp{Success: true}, "")
}

// handleResetElection resets our state and starts a new election
func (e *node) handleResetElection(rpc RPCRequest) {
	e.log.Debug("RPC: election.ResetElectionReq{}")
	e.setLeader("")
	e.state = candidateState
	rpc.respond(rpc.RPC, ResetElectionResp{}, "")
}

// handleHeartBeat handles heartbeat requests from the elected leader
func (e *node) handleHeartBeat(rpc RPCRequest, req HeartBeatReq) {
	e.log.Debugf("RPC: %#v", req)
	resp := HeartBeatResp{
		From: e.self,
		Term: e.currentTerm,
	}

	defer func() {
		rpc.respond(rpc.RPC, resp, "")
	}()

	// This might occur if 2 or more nodes think they are elected leader. In this
	// case all leaders that emit heartbeats will both fall back to follower, from
	// there the followers will timeout waiting for a heartbeat and the vote will
	// occur again, hopefully this time without electing 2 leaders.
	//
	// It's also possible that one leader sends it's heartbeats before the other leader,
	// in that case the first leader to send a heartbeat becomes leader.
	//
	// This can also occur if a leader loses connectivity to the rest of the cluster.
	// In this case we become the follower of who ever sent us a heartbeat, regardless
	// of our current term compared the one who sent us the heartbeat.
	if e.state != followerState {
		e.state = followerState
		e.currentTerm = req.Term
		resp.Term = req.Term
	}

	// Only the node with the most votes is the leader and should report heartbeats
	e.setLeader(req.From)

	e.lastContact = time.Now()
}

// handleVote determines who we will vote for this term
func (e *node) handleVote(rpc RPCRequest, req VoteReq) {
	e.log.Debugf("RPC: %#v", req)
	resp := VoteResp{
		Term:      e.currentTerm,
		Candidate: e.self,
		Granted:   false,
	}

	defer func() {
		rpc.respond(rpc.RPC, resp, "")
	}()

	// Check if we have an existing leader (who's not the candidate). Votes are rejected
	// if there is a known leader. If a leader wants to step down, they notify followers
	// with the ResetElection RPC call.
	if e.leader != "" && e.leader != req.Candidate {
		e.log.Debugf("rejecting vote request from '%s' since we have leader '%s'", req.Candidate, e.leader)
		return
	}

	// Ignore an older term
	if req.Term < e.currentTerm {
		return
	}

	// Increase the term if we see a newer one
	if req.Term > e.currentTerm {
		// Ensure transition to follower
		e.log.Debugf("received a vote request with a newer term '%d'", req.Term)
		e.state = followerState
		e.currentTerm = req.Term
		resp.Term = req.Term
	}

	// Check if we've voted in this election before
	if e.vote.LastTerm == req.Term && e.vote.LastCandidate != "" {
		e.log.Debugf("ignoring vote request from '%s'; already voted for '%s' in election '%d'",
			req.Candidate, e.vote.LastCandidate, req.Term)
		if e.vote.LastCandidate == req.Candidate {
			e.log.Debugf("duplicate requestVote from candidate '%s'", req.Candidate)
			resp.Granted = true
		}
		return
	}

	// Always vote for the first candidate we receive a request from for this term
	e.vote.LastTerm = req.Term
	e.vote.LastCandidate = req.Candidate

	// Tell the requester we voted for him
	resp.Granted = true
	e.lastContact = time.Now()
	return
}

func (e *node) handleSetPeers(rpc RPCRequest, req SetPeersReq) {
	e.log.Debugf("RPC: %#v", req)
	e.peers = req.Peers
	rpc.respond(rpc.RPC, SetPeersResp{}, "")
}

func (e *node) handleGetState(rpc RPCRequest) {
	e.log.Debug("RPC: election.GetStateReq{}")

	rpc.respond(rpc.RPC, GetStateResp{
		Peers:  e.peers,
		State:  e.state.String(),
		Leader: e.leader,
	}, "")
}

func (e *node) quorumSize() int {
	size := len(e.peers)
	if size == 0 {
		return 1
	}
	return size/2 + 1
}

func (e *node) send(req interface{}) chan RPCResponse {
	respCh := make(chan RPCResponse, 1)

	select {
	case e.rpcCh <- RPCRequest{
		Request:  req,
		respChan: respCh,
	}:
	// Avoid blocking if the rpcCh is full
	default:
		e.conf.Log.Error("RPC send failed; rpc channel is full")
		respCh <- RPCResponse{}
	}
	return respCh
}

// randomDuration returns a value that is between the minDur and 2x minDur.
func randomDuration(minDur time.Duration) time.Duration {
	return minDur + time.Duration(rand.Int63())%minDur
}

// WaitForConnect waits for the specified address to accept connections then returns nil.
// Returns an error if all attempts have been exhausted.
func WaitForConnect(address string, attempts int, interval time.Duration) error {
	var err error
	var conn net.Conn
	for i := 0; i < attempts; i++ {
		conn, err = net.Dial("tcp", address)
		if err != nil {
			continue
		}
		conn.Close()
		time.Sleep(interval)
		return nil
	}
	return fmt.Errorf("while connecting to '%s' - '%s' after %d attempts", address, err, attempts)
}
