package election_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"testing"

	"github.com/mailgun/holster/v3/election"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func sendRPC(ctx context.Context, peer string, req election.RPCRequest, resp *election.RPCResponse) error {
	// Marshall the RPC request to json
	b, err := json.Marshal(req)
	if err != nil {
		return errors.Wrap(err, "while encoding request")
	}

	// Create a new http request with context
	hr, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/rpc", peer), bytes.NewBuffer(b))
	if err != nil {
		return errors.Wrap(err, "while creating request")
	}
	hr.WithContext(ctx)

	// Send the request
	hp, err := http.DefaultClient.Do(hr)
	if err != nil {
		return errors.Wrap(err, "while sending http request")
	}

	// Decode the response from JSON
	dec := json.NewDecoder(hp.Body)
	if err := dec.Decode(&resp); err != nil {
		return errors.Wrap(err, "while decoding response")
	}
	return nil
}

func newHandler(node election.Node) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		var req election.RPCRequest
		if err := dec.Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
		}
		var resp election.RPCResponse
		node.ReceiveRPC(req, &resp)

		enc := json.NewEncoder(w)
		if err := enc.Encode(resp); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
	}
}

// This example spawns 2 nodes, in a real application you would
// only spawn a single node which would represent your application
// in the election.
func SimpleExample(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	node1, err := election.SpawnNode(election.Config{
		// A list of known peers at startup
		Peers: []string{"localhost:7080", "localhost:7081"},
		// A unique identifier used to identify us in a list of peers
		Self: "localhost:7080",
		// Called whenever the library detects a change in leadership
		Observer: func(leader string) {
			log.Printf("Current Leader: %s\n", leader)
		},
		// Called when the library wants to contact other peers
		SendRPC: sendRPC,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer node1.Close()

	node2, err := election.SpawnNode(election.Config{
		Peers:   []string{"localhost:7080", "localhost:7081"},
		Self:    "localhost:7081",
		SendRPC: sendRPC,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer node2.Close()

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/rpc", newHandler(node1))
		log.Fatal(http.ListenAndServe(":7080", mux))
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", newHandler(node2))
	log.Fatal(http.ListenAndServe(":7081", mux))
}
