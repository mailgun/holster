package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/mailgun/holster/v3/discovery"
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

		// Example of how a peer might exclude RPC
		// commands it doesn't want made.
		if req.RPC == election.SetPeersRPC {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprintf("RPC request '%s' not allowed", req.RPC)))
			return
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

func main() {
	if len(os.Args) != 4 {
		logrus.Fatal("usage: <election-address:8080> <memberlist-address:8180> <known-address:8180>")
	}

	electionAddr, memberListAddr, knownAddr := os.Args[1], os.Args[2], os.Args[3]
	//logrus.SetLevel(logrus.DebugLevel)

	node, err := election.NewNode(election.Config{
		// A unique identifier used to identify us in a list of peers
		UniqueID: electionAddr,
		// Called whenever the library detects a change in leadership
		OnUpdate: func(leader string) {
			logrus.Printf("Current Leader: %s\n", leader)
		},
		// Called when the library wants to contact other peers
		SendRPC: sendRPC,
	})
	if err != nil {
		logrus.Fatal(err)
	}

	// Create a member list catalog
	ml, err := discovery.NewMemberList(context.Background(), discovery.MemberListConfig{
		BindAddress: memberListAddr,
		Peer: discovery.Peer{
			ID:       uuid.New().String(),
			Metadata: []byte(electionAddr),
		},
		KnownPeers: []string{knownAddr},
		OnUpdate: func(peers []discovery.Peer) {
			var result []string
			for _, p := range peers {
				result = append(result, string(p.Metadata))
			}
			logrus.Infof("Update Peers: %s", result)
			node.SetPeers(result)
		},
	})
	if err != nil {
		logrus.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", newHandler(node))
	go func() {
		logrus.Fatal(http.ListenAndServe(electionAddr, mux))
	}()

	// Wait until the http server is up and can receive RPC requests
	if err := election.WaitForConnect(electionAddr, 10, time.Millisecond*100); err != nil {
		logrus.Fatal(err)
	}

	// Now that our http handler is listening for requests we
	// can safely start the election.
	node.Start()

	// Wait here for signals to clean up our mess
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	for range c {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		if err := ml.Close(ctx); err != nil {
			logrus.WithError(err).Error("during member list catalog close")
		}
		cancel()
		node.Stop()
		os.Exit(0)
	}
}
