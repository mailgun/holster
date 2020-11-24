package election_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mailgun/holster/v3/election"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRPCRequest(t *testing.T) {
	out := election.RPCRequest{
		RPC: election.HeartBeatRPC,
		Request: election.HeartBeatReq{
			From: "node1",
			Term: 1,
		},
	}

	b, err := json.Marshal(out)
	require.NoError(t, err)
	assert.Equal(t, `{"rpc":"heartbeat","request":{"from":"node1","term":1}}`, string(b))

	var in election.RPCRequest
	err = json.Unmarshal(b, &in)
	require.NoError(t, err)
	assert.Equal(t, out, in)
}

func TestRPCResponse(t *testing.T) {
	out := election.RPCResponse{
		RPC: election.HeartBeatRPC,
		Response: election.HeartBeatResp{
			From:    "node1",
			Success: true,
			Term:    1,
		},
	}
	b, err := json.Marshal(out)
	require.NoError(t, err)
	assert.Equal(t, `{"rpc":"heartbeat","response":{"from":"node1","term":1,"success":true}}`, string(b))

	var in election.RPCResponse
	err = json.Unmarshal(b, &in)
	require.NoError(t, err)
	assert.Equal(t, out, in)
}

func TestHTTPServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in election.RPCRequest

		b, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(b, &in))
		var resp election.RPCResponse

		switch in.RPC {
		case election.HeartBeatRPC:
			resp = election.RPCResponse{
				RPC: election.HeartBeatRPC,
				Response: election.HeartBeatResp{
					From:    "node1",
					Term:    10,
					Success: true,
				},
			}
		default:
			resp = election.RPCResponse{
				Error: fmt.Sprintf("invalid rpc request '%s'", in.RPC),
			}
		}
		out, err := json.Marshal(resp)
		require.NoError(t, err)
		w.Write(out)
	}))
	defer ts.Close()

	// Marshall our request
	b, err := json.Marshal(election.RPCRequest{
		RPC: election.HeartBeatRPC,
		Request: election.HeartBeatReq{
			Term: 10,
			From: "node10",
		},
	})
	require.NoError(t, err)

	// Send the request to the server
	req, err := http.NewRequest(http.MethodPost, ts.URL, bytes.NewBuffer(b))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	// Unmarshall the response
	var rpcResp election.RPCResponse
	b, err = ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(b, &rpcResp)
	require.NoError(t, err)

	// Should have the response we expect
	hb := rpcResp.Response.(election.HeartBeatResp)
	assert.Equal(t, uint64(10), hb.Term)
	assert.Equal(t, "node1", hb.From)
	assert.Equal(t, true, hb.Success)

	// Send an unknown rpc request to the server
	req, err = http.NewRequest(http.MethodPost, ts.URL, bytes.NewBuffer([]byte(`{"rpc":"unknown"}`)))
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)

	// Unmarshall the response
	b, err = ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(b, &rpcResp)
	require.NoError(t, err)

	// Should have the response we expect
	assert.Equal(t, "invalid rpc request 'unknown'", rpcResp.Error)
}
