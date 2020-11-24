package election

import (
	"encoding/json"
)

type VoteResp struct {
	// The address of the candidate
	Candidate string `json:"candidate"`
	// Newer term if leader is out of date.
	Term uint64 `json:"term"`
	// Is the vote granted.
	Granted bool `json:"granted"`
}

type VoteReq struct {
	// The address of the candidate
	Candidate string `json:"candidate-address"`
	// Newer term if leader is out of date.
	Term uint64 `json:"term"`
}

type ResetElectionReq struct{}
type ResetElectionResp struct{}

type ResignReq struct{}
type ResignResp struct {
	Success bool `json:"success"`
}

type HeartBeatReq struct {
	From string `json:"from"`
	Term uint64 `json:"term"`
}
type HeartBeatResp struct {
	From    string `json:"from"`
	Term    uint64 `json:"term"`
	Success bool   `json:"success"`
}

type RPC string

const (
	HeartBeatRPC     = RPC("heartbeat")
	VoteRPC          = RPC("vote")
	ResetElectionRPC = RPC("reset-election")
	UnknownRPC       = RPC("unknown")
)

type RPCPayload struct {
	RPC      RPC             `json:"rpc"`
	Request  json.RawMessage `json:"request,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type RPCResponse struct {
	RPC      RPC
	Response interface{}
	Error    string
}

func (r *RPCResponse) UnmarshalJSON(s []byte) error {
	var in RPCPayload
	if err := json.Unmarshal(s, &in); err != nil {
		return err
	}
	r.Error = in.Error
	r.RPC = in.RPC

	switch in.RPC {
	case HeartBeatRPC:
		resp := HeartBeatResp{}
		if err := json.Unmarshal(in.Response, &resp); err != nil {
			return err
		}
		r.Response = resp
	}
	return nil
}

func (r RPCResponse) MarshalJSON() ([]byte, error) {
	out, err := json.Marshal(r.Response)
	if err != nil {
		return nil, err
	}
	p := RPCPayload{
		Error:    r.Error,
		RPC:      r.RPC,
		Response: out,
	}
	return json.Marshal(p)
}

type RPCRequest struct {
	RPC      RPC
	Request  interface{}
	respChan chan RPCResponse
}

func (r RPCRequest) MarshalJSON() ([]byte, error) {
	out, err := json.Marshal(r.Request)
	if err != nil {
		return nil, err
	}
	p := RPCPayload{
		RPC:     r.RPC,
		Request: out,
	}
	return json.Marshal(p)
}

func (r *RPCRequest) UnmarshalJSON(s []byte) error {
	var in RPCPayload
	if err := json.Unmarshal(s, &in); err != nil {
		return err
	}
	r.RPC = in.RPC

	switch in.RPC {
	case HeartBeatRPC:
		req := HeartBeatReq{}
		if err := json.Unmarshal(in.Request, &req); err != nil {
			return err
		}
		r.Request = req
	}
	return nil
}

// respond is used to respond with a response, error or both
func (r *RPCRequest) respond(rpc RPC, resp interface{}, errorMsg string) {
	r.respChan <- RPCResponse{RPC: rpc, Response: resp, Error: errorMsg}
}
