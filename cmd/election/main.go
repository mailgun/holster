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
	"github.com/mailgun/holster/v4/discovery"
	"github.com/mailgun/holster/v4/election"
	"github.com/mailgun/holster/v4/errors"
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
	defer func() {
		_ = hp.Body.Close()
	}()

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
			status := http.StatusBadRequest
			w.WriteHeader(status)
			if _, err := w.Write([]byte(err.Error())); err != nil {
				logrus.WithError(err).WithField("status", status).Warn("while writing response")
			}
		}

		// Example of how a peer might exclude RPC
		// commands it doesn't want made.
		if req.RPC == election.SetPeersRPC {
			status := http.StatusBadRequest
			w.WriteHeader(status)
			if _, err := fmt.Fprintf(w, "RPC request '%s' not allowed", req.RPC); err != nil {
				logrus.WithError(err).WithField("status", status).Warn("while writing response")
			}
			return
		}

		var resp election.RPCResponse
		node.ReceiveRPC(req, &resp)

		enc := json.NewEncoder(w)
		if err := enc.Encode(resp); err != nil {
			status := http.StatusInternalServerError
			w.WriteHeader(status)
			if _, err = w.Write([]byte(err.Error())); err != nil {
				logrus.WithError(err).WithField("status", status).Warn("while writing response")
			}
		}
	}
}

func main() {
	if len(os.Args) != 4 {
		logrus.Fatal("usage: <election-address:8080> <memberlist-address:8180> <known-address:8180>")
	}

	electionAddr, memberListAddr, knownAddr := os.Args[1], os.Args[2], os.Args[3]
	// logrus.SetLevel(logrus.DebugLevel)

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
			if err := node.SetPeers(context.Background(), result); err != nil {
				logrus.WithError(err).Warn("while setting peers")
			}
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
	if err := node.Start(context.Background()); err != nil {
		logrus.Fatal(err)
	}

	// Wait here for signals to clean up our mess
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	for range c {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		if err := ml.Close(ctx); err != nil {
			logrus.WithError(err).Error("during member list catalog close")
		}
		cancel()
		_ = node.Stop(context.Background())
		os.Exit(0)
	}
}
