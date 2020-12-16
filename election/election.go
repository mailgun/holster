package election

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/mailgun/holster/v3/setter"
	"github.com/mailgun/holster/v3/slice"
	"github.com/mailgun/holster/v3/syncutil"
	"github.com/sirupsen/logrus"
)

type State uint32

const (
	// FollowerState means we are following the leader and expect
	// to get heart beats regularly. This is the initial state, as
	// we don't want to force an election when a new node joins
	// the cluster.
	FollowerState State = iota
	// CandidateState means we are actively attempting to become leader
	CandidateState
	// LeaderState means we have received a quorum of votes while
	// in CandidateState and have assumed leadership.
	LeaderState
	// ShutdownState means we are in the process of shutting down
	ShutdownState
)

func (s State) String() string {
	switch s {
	case FollowerState:
		return "Follower"
	case CandidateState:
		return "Candidate"
	case LeaderState:
		return "Leader"
	case ShutdownState:
		return "Shutdown"
	default:
		return "Unknown"
	}
}

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

	// The Initial list of peers to be considered in the election, including ourself.
	Peers []string

	// The name this peer identifies itself as, as found in the Peers list
	Name string

	// Called when the leader changes
	Observer Observer

	// The logger used errors and warning
	Log logrus.FieldLogger

	// Sends an RPC request to a peer, This function must be provided and can
	// utilize any network communication the implementer wishes. If context cancelled
	// should return an error.
	SendRPC func(context.Context, string, RPCRequest, *RPCResponse) error
}

type Observer func(string)

type Node interface {
	// Set the list of peers to be considered for the election, this list MUST
	// include ourself as defined by `Config.Name`.
	SetPeers([]string) error

	// If leader, resigns as leader and starts a new election that we will not
	// participate in.
	Resign() bool

	// Returns true if we are currently leader
	IsLeader() bool

	// Returns the current leader
	Leader() string

	// Returns the current state of this node
	State() State

	// Called
	ReceiveRPC(RPCRequest, *RPCResponse)

	// Cancels the election, resigns if we are leader and waits for all go
	// routines to complete before returning.
	Close()
}

type node struct {
	conf  Config // The election configuration
	state State  // Current state of our node
	vote  struct {
		CurrentTerm   uint64
		LastTerm      uint64
		LastCandidate string
	} // Current state of the vote
	currentTerm uint64          // The current term of the election when in candidate state
	rpcCh       chan RPCRequest // RPC Response channel, listen for for RPC responses on this channel
	self        string          // Our name
	lock        sync.RWMutex    // lock for peers and leader
	peers       []string
	leader      string
	lastContact time.Time     // The last successful contact with the leader (if we are a follower)
	shutdownCh  chan struct{} // Signals we are in shutdown
	log         logrus.FieldLogger
	wg          syncutil.WaitGroup
}

// Spawns a new node that will participate in the election.
func SpawnNode(conf Config) (Node, error) {

	if conf.Name == "" {
		return nil, errors.New("refusing to spawn a new node with no Config.Name defined")
	}

	setter.SetDefault(&conf.Log, logrus.WithField("name", conf.Name))
	setter.SetDefault(&conf.LeaderQuorumTimeout, time.Second*30)
	setter.SetDefault(&conf.HeartBeatTimeout, time.Second*5)
	setter.SetDefault(&conf.ElectionTimeout, time.Second*10)
	setter.SetDefault(&conf.NetworkTimeout, time.Second*2)

	c := &node{
		shutdownCh: make(chan struct{}),
		rpcCh:      make(chan RPCRequest, 5_000),
		self:       conf.Name,
		conf:       conf,
		log:        conf.Log,
	}
	c.wg.Go(c.run)
	return c, c.SetPeers(conf.Peers)
}

// Called by the implementer when an RPC is received from another node
func (e *node) ReceiveRPC(req RPCRequest, resp *RPCResponse) {
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
func (e *node) SetPeers(peers []string) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.peers = peers
	return nil
}

// GetPeers returns the current peers this node knows about.
func (e *node) GetPeers() []string {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.peers
}

// State returns the current state of this node
func (e *node) State() State {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.state
}

func (e *node) setState(state State) {
	e.log.Debugf("State Change (%s)", state)
	e.lock.RLock()
	defer e.lock.RUnlock()
	e.state = state
}

// IsLeader returns true if this node was elected leader
func (e *node) IsLeader() bool {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.self == e.leader
}

// Leader returns the name of the node that is currently leader
func (e *node) Leader() string {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.leader
}

func (e *node) setLeader(leader string) {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.leader != leader {
		e.log.Debugf("Set Leader (%s)", leader)
		e.leader = leader
		if e.conf.Observer != nil {
			e.conf.Observer(leader)
		}
	}
}

// Resign will cause this node to step down as leader, if this
// node is NOT leader, this does nothing and returns 'false'
func (e *node) Resign() bool {
	respCh := make(chan RPCResponse, 1)
	e.rpcCh <- RPCRequest{
		Request:  ResignReq{},
		respChan: respCh,
	}

	select {
	case rpcResp := <-respCh:
		resp, ok := rpcResp.Response.(ResignResp)
		if !ok {
			return false
		}
		if rpcResp.Error != "" {
			return false
		}
		return resp.Success
	case <-e.shutdownCh:
		return false
	}
}

// Close closes all internal go routines and if this node is currently
// leader, resigns as leader.
func (e *node) Close() {
	close(e.shutdownCh)
	e.wg.Wait()
}

func (e *node) run() {
	for {
		e.log.Debug("main loop")
		select {
		case <-e.shutdownCh:
			e.setLeader("")
			e.setState(ShutdownState)
			return
		default:
		}

		switch e.state {
		case FollowerState:
			e.runFollower()
		case CandidateState:
			e.runCandidate()
		case LeaderState:
			e.runLeader()
		}
	}
}

func (e *node) runFollower() {
	e.log.Debugf("entering follower state, current leader is '%s'", e.Leader())
	heartbeatTimer := time.NewTicker(randomDuration(e.conf.HeartBeatTimeout))

	for e.state == FollowerState {
		select {
		case rpc := <-e.rpcCh:
			e.processRPC(rpc)
		case <-heartbeatTimer.C:

			// Check if we have had successful contact with the leader
			if time.Now().Sub(e.lastContact) < e.conf.HeartBeatTimeout {
				continue
			}

			// Heartbeat failed! Transition to the candidate state
			e.log.Debugf("heartbeat timeout, starting election; previous leader was '%s'", e.Leader())
			e.setLeader("")
			e.setState(CandidateState)
			return
		case <-e.shutdownCh:
			heartbeatTimer.Stop()
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
	// We re-start the vote if we have not received a heart beat from a chosen leader before
	// this timer expires.
	electionTimer := time.NewTimer(randomDuration(e.conf.ElectionTimeout))

	// Tally the votes, need a simple majority
	grantedVotes := 0
	votesNeeded := e.quorumSize()
	e.log.Debugf("votes needed: %d", votesNeeded)

	for e.State() == CandidateState {
		select {
		case <-voteTimer.C:
			voteCh = e.electSelf()
			voteTimer.Stop()
		case rpc := <-e.rpcCh:
			e.processRPC(rpc)
		case vote := <-voteCh:
			// Check if the term is greater than ours, bail
			if vote.Term > e.currentTerm {
				e.log.Debug("newer term discovered, fallback to follower")
				e.state = FollowerState
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
				e.state = LeaderState
				e.setLeader(e.self)
				return
			}
		case <-electionTimer.C:
			// Election failed! Restart the election. We simply return, which will kick us back into runCandidate
			e.log.Debug("Election timeout reached, restarting election")
			electionTimer.Stop()
			return
		case <-e.shutdownCh:
			return
		}
	}
}

// electSelf is used to send a SendVote() RPC to all peers with a vote for
// ourself. This has the side affecting of incrementing the current term. The
// response channel returned is used to wait for all the responses, including a
// vote for ourself.
func (e *node) electSelf() <-chan VoteResp {
	peers := e.GetPeers()
	respCh := make(chan VoteResp, len(peers))

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
	for _, peer := range peers {
		if peer == e.self {
			continue
		}
		askPeer(peer, e.currentTerm, e.self)
	}
	return respCh
}

func (e *node) runLeader() {
	quorumTicker := time.NewTicker(e.conf.LeaderQuorumTimeout)
	heartBeatTicker := time.NewTicker(randomDuration(e.conf.HeartBeatTimeout / 10))
	heartBeatReplyCh := make(chan HeartBeatResp, 5_000)
	peersLastContact := make(map[string]time.Time, len(e.GetPeers()))

	for e.state == LeaderState {
		select {
		case rpc := <-e.rpcCh:
			e.processRPC(rpc)
		case reply := <-heartBeatReplyCh:
			// Is the reply from a peer we are familiar with?
			if !slice.ContainsString(reply.From, e.GetPeers(), nil) {
				e.log.WithField("peer", reply.From).
					Debug("leader received heartbeat reply from peer not in our peer list; ignoring")
				break
			}
			peersLastContact[reply.From] = time.Now()
		case <-heartBeatTicker.C:
			for _, peer := range e.GetPeers() {
				e.sendHeartBeat(peer, heartBeatReplyCh)
			}
		case <-quorumTicker.C:
			// Check if we have received contact from a quorum of nodes within the leader quorum timeout interval.
			// If not, we step down as we may have lost connectivity.
			contacted := 0
			now := time.Now()
			for _, peer := range e.GetPeers() {
				if peer == e.self {
					contacted++
					continue
				}

				lc, ok := peersLastContact[peer]
				if !ok {
					continue
				}
				diff := now.Sub(lc)
				if diff >= e.conf.HeartBeatTimeout {
					e.log.Debugf("no heartbeat response from '%s' for '%s'", peer, diff)
					continue
				}
				contacted++
			}

			// Verify we can contact a quorum
			quorum := e.quorumSize()
			if contacted < quorum {
				e.log.Debug("failed to receive heart beats from a quorum of peers; stepping down")
				e.state = FollowerState
				// TODO: Perhaps we send ResetElection to what peers we can?
				//  This would avoid having to wait for the heartbeat timeout
				//  to start a new election.
			}
		case <-e.shutdownCh:
			e.state = ShutdownState
			heartBeatTicker.Stop()
			quorumTicker.Stop()
			if e.IsLeader() {
				// Notify all followers we are no longer leader
				for _, peer := range e.GetPeers() {
					e.sendElectionReset(peer)
				}
			}
		}
	}
	e.lastContact = time.Now()
	if e.IsLeader() {
		e.setLeader("")
	}
	quorumTicker.Stop()
}

func (e *node) sendHeartBeat(peer string, heartBeatReplyCh chan HeartBeatResp) {
	// Don't heartbeat ourself
	if peer == e.self {
		return
	}

	e.wg.Go(func() {
		var resp RPCResponse
		req := RPCRequest{
			RPC: HeartBeatRPC,
			Request: HeartBeatReq{
				From: e.self,
				Term: e.currentTerm,
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
	// TODO: Should check for state = shutdown?
	switch cmd := rpc.Request.(type) {
	case VoteReq:
		e.handleVote(rpc, cmd)
	case ResetElectionReq:
		e.handleResetElection(rpc)
	case HeartBeatReq:
		e.handleHeartBeat(rpc, cmd)
	case ResignReq:
		e.handleResign(rpc)
	default:
		e.log.Errorf("got unexpected command %#v", rpc.Request)
		rpc.respond(rpc.RPC, nil, "unexpected command")
	}
}

// handleResign Notifies all followers that we are stepping down as leader.
// if we are leader returns Success = true
func (e *node) handleResign(rpc RPCRequest) {
	e.setLeader("")
	e.state = FollowerState
	for _, peer := range e.GetPeers() {
		e.sendElectionReset(peer)
	}
	rpc.respond(rpc.RPC, ResignReq{}, "")
}

// handleResetElection resets our state and starts a new election
func (e *node) handleResetElection(rpc RPCRequest) {
	e.setLeader("")
	e.state = CandidateState
	rpc.respond(rpc.RPC, ResetElectionResp{}, "")
}

// handleHeartBeat handles heartbeat requests from the elected leader
func (e *node) handleHeartBeat(rpc RPCRequest, req HeartBeatReq) {
	resp := HeartBeatResp{
		From: e.self,
		Term: e.currentTerm,
	}

	defer func() {
		rpc.respond(rpc.RPC, resp, "")
	}()

	// Ignore an older term
	if req.Term < e.currentTerm {
		return
	}

	// This might occur if 2 or more nodes think they are elected leader. In this
	// case all leaders that emit heartbeats will both fall back to follower, from
	// there the followers will timeout waiting for a heartbeat and the vote will
	// occur again, hopefully this time without electing 2 leaders.
	//
	// It's also possible that one leader sends it's heartbeats before the other leader,
	// in that case the first leader to send a heartbeat becomes leader.
	//
	// This can also occur if a follower loses connectivity to the rest of the cluster.
	// In this case we become the follower of who ever sent us the heartbeat.
	if e.state != FollowerState {
		e.state = FollowerState
		e.currentTerm = req.Term
		resp.Term = req.Term
	}

	// Only the node with the most votes is the leader and should report heartbeats
	e.setLeader(req.From)

	e.lastContact = time.Now()
}

// handleVote determines who we will vote for this term
func (e *node) handleVote(rpc RPCRequest, req VoteReq) {
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
	leader := e.Leader()
	if leader != "" && leader != req.Candidate {
		e.log.Debugf("rejecting vote request from '%s' since we have leader '%s'", req.Candidate, leader)
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
		e.setState(FollowerState)
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

func (e *node) quorumSize() int {
	size := len(e.GetPeers())
	if size == 0 {
		return 1
	}
	return size/2 + 1
}

// randomDuration returns a value that is between the minDur and 2x minDur.
func randomDuration(minDur time.Duration) time.Duration {
	return minDur + time.Duration(rand.Int63())%minDur
}
