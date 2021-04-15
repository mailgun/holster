package etcdutil_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy"
	etcd "github.com/coreos/etcd/clientv3"
	"github.com/mailgun/holster/v3/clock"
	"github.com/mailgun/holster/v3/etcdutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestElection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	election, err := etcdutil.NewElection(ctx, client, etcdutil.ElectionConfig{
		EventObserver: func(e etcdutil.ElectionEvent) {
			if e.Err != nil {
				t.Fatal(e.Err.Error())
			}
		},
		Election:  "/my-election",
		Candidate: "me",
	})
	require.Nil(t, err)

	assert.Equal(t, true, election.IsLeader())
	election.Close()
	assert.Equal(t, false, election.IsLeader())
}

func TestTwoCampaigns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	logrus.SetLevel(logrus.DebugLevel)

	c1, err := etcdutil.NewElection(ctx, client, etcdutil.ElectionConfig{
		EventObserver: func(e etcdutil.ElectionEvent) {
			if e.Err != nil {
				t.Fatal(e.Err.Error())
			}
		},
		Election:  "/my-election",
		Candidate: "c1",
	})
	require.Nil(t, err)

	c2Chan := make(chan etcdutil.ElectionEvent, 5)
	c2, err := etcdutil.NewElection(ctx, client, etcdutil.ElectionConfig{
		EventObserver: func(e etcdutil.ElectionEvent) {
			if err != nil {
				t.Fatal(err.Error())
			}
			c2Chan <- e
		},
		Election:  "/my-election",
		Candidate: "c2",
	})
	require.Nil(t, err)

	assert.Equal(t, true, c1.IsLeader())
	assert.Equal(t, false, c2.IsLeader())

	// Cancel first candidate
	c1.Close()
	assert.Equal(t, false, c1.IsLeader())

	// Second campaign should become leader
	e := <-c2Chan
	assert.Equal(t, false, e.IsLeader)
	e = <-c2Chan
	assert.Equal(t, true, e.IsLeader)
	assert.Equal(t, false, e.IsDone)

	c2.Close()
	e = <-c2Chan
	assert.Equal(t, false, e.IsLeader)
	assert.Equal(t, false, e.IsDone)

	e = <-c2Chan
	assert.Equal(t, false, e.IsLeader)
	assert.Equal(t, true, e.IsDone)
}

func TestElectionsSuite(t *testing.T) {
	etcdCAPath := os.Getenv("ETCD3_CA")
	if etcdCAPath != "" {
		t.Skip("Tests featuring toxiproxy cannot deal with TLS")
	}
	suite.Run(t, new(ElectionsSuite))
}

type ElectionsSuite struct {
	suite.Suite
	toxiProxies    []*toxiproxy.Proxy
	proxiedClients []*etcd.Client
}

func (s *ElectionsSuite) SetupTest() {
	etcdEndpoint := os.Getenv("ETCD3_ENDPOINT")
	if etcdEndpoint == "" {
		etcdEndpoint = "127.0.0.1:2379"
	}

	s.toxiProxies = make([]*toxiproxy.Proxy, 2)
	s.proxiedClients = make([]*etcd.Client, 2)
	for i := range s.toxiProxies {
		toxiProxy := toxiproxy.NewProxy()
		toxiProxy.Name = fmt.Sprintf("etcd_clt_%d", i)
		toxiProxy.Upstream = etcdEndpoint
		s.Require().Nil(toxiProxy.Start())
		s.toxiProxies[i] = toxiProxy

		var err error
		// Make sure to access proxy via 127.0.0.1 otherwise TLS verification fails.
		proxyEndpoint := toxiProxy.Listen
		if strings.HasPrefix(proxyEndpoint, "[::]:") {
			proxyEndpoint = "127.0.0.1:" + proxyEndpoint[5:]
		}
		s.proxiedClients[i], err = etcd.New(etcd.Config{
			Endpoints:   []string{proxyEndpoint},
			DialTimeout: 1 * clock.Second,
		})
		s.Require().Nil(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*clock.Second)
	defer cancel()
	_, err := s.proxiedClients[0].Delete(ctx, "/elections", etcd.WithPrefix())
	s.Require().Nil(err)
}

func (s *ElectionsSuite) TearDownTest() {
	for _, proxy := range s.toxiProxies {
		proxy.Stop()
	}
	for _, etcdClt := range s.proxiedClients {
		_ = etcdClt.Close()
	}
}

// When the leader is stopped then another candidate is elected.
func (s *ElectionsSuite) TestLeaderStops() {
	campaign := "LeadershipTransferOnStop"
	e0, ch0 := s.newElection(campaign, 0)
	s.assertElectionWinner(ch0, 3*clock.Second)

	e1, ch1 := s.newElection(campaign, 1)
	defer e1.Close()
	s.assertElectionLooser(ch1, 200*clock.Millisecond)

	// When
	e0.Close()

	// Then
	s.assertElectionWinner(ch1, 3*clock.Second)
}

// A candidate may never be elected.
func (s *ElectionsSuite) TestNeverElected() {
	campaign := "NeverElected"
	e0, ch0 := s.newElection(campaign, 0)
	defer e0.Close()
	s.assertElectionWinner(ch0, 3*clock.Second)

	e1, ch1 := s.newElection(campaign, 1)
	s.assertElectionLooser(ch1, 200*clock.Millisecond)

	// When
	e1.Close()

	// Then
	s.assertElectionClosed(ch1, 200*clock.Millisecond)
}

// When the leader is loosing connection with etcd, then another candidate gets
// promoted.
func (s *ElectionsSuite) TestLeaderConnLost() {
	campaign := "LeadershipLost"
	e0, ch0 := s.newElection(campaign, 0)
	defer e0.Close()
	s.assertElectionWinner(ch0, 3*clock.Second)

	e1, ch1 := s.newElection(campaign, 1)
	defer e1.Close()
	s.assertElectionLooser(ch1, 200*clock.Millisecond)

	// When
	s.toxiProxies[0].Stop()

	// Then
	s.assertElectionLooser(ch0, 5*clock.Second)
	s.assertElectionWinner(ch1, 5*clock.Second)
}

// It is possible to stop a former leader while it is trying to reconnect with
// Etcd.
func (s *ElectionsSuite) TestLostLeaderStop() {
	campaign := "LostLeaderStop"
	e0, ch0 := s.newElection(campaign, 0)
	s.assertElectionWinner(ch0, 3*clock.Second)

	e1, ch1 := s.newElection(campaign, 1)
	defer e1.Close()
	s.assertElectionLooser(ch1, 200*clock.Millisecond)

	// Given
	s.toxiProxies[0].Stop()
	clock.Sleep(2 * clock.Second)

	// When
	e0.Close()

	// Then
	s.assertElectionClosed(ch0, 3*clock.Second)
}

// FIXME: This test gets stuck on e0.Stop().
//// If Etcd is down on start the candidate keeps trying to connect.
//func (s *ElectionsSuite) TestEtcdDownOnStart() {
//	s.toxiProxies[0].Stop()
//	campaign := "EtcdDownOnStart"
//	e0, ch0 := s.newElection(campaign, 0)
//
//	// When
//	_ = s.toxiProxies[0].Start()
//
//	// Then
//	s.assertElectionWinner(ch0, 3*clock.Second)
//	e0.Stop()
//}

// If provided etcd endpoint candidate keeps trying to connect until it is
// stopped.
func (s *ElectionsSuite) TestBadEtcdEndpoint() {
	s.toxiProxies[0].Stop()
	campaign := "/BadEtcdEndpoint"
	e0, ch0 := s.newElection(campaign, 0)

	// When
	e0.Close()

	// Then
	s.assertElectionClosed(ch0, 3*clock.Second)
}

func (s *ElectionsSuite) assertElectionWinner(ch chan bool, timeout clock.Duration) {
	timeoutCh := clock.After(timeout)
	for {
		select {
		case elected := <-ch:
			if elected {
				return
			}
		case <-timeoutCh:
			s.Fail("Timeout waiting for election winning")
		}
	}
}

func (s *ElectionsSuite) assertElectionLooser(ch chan bool, timeout clock.Duration) {
	timeoutCh := clock.After(timeout)
	for {
		select {
		case elected := <-ch:
			if !elected {
				return
			}
		case <-timeoutCh:
			s.Fail("Timeout waiting for election loss")
		}
	}
}

func (s *ElectionsSuite) assertElectionClosed(ch chan bool, timeout clock.Duration) {
	timeoutCh := clock.After(timeout)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-timeoutCh:
			s.Fail("Timeout waiting for election closed")
		}
	}
}

func (s *ElectionsSuite) newElection(campaign string, id int) (*etcdutil.Election, chan bool) {
	electedCh := make(chan bool, 32)
	candidate := fmt.Sprintf("candidate-%d", id)
	electionCfg := etcdutil.ElectionConfig{
		EventObserver: func(e etcdutil.ElectionEvent) {
			logrus.Infof("%s got %#v", candidate, e)
			if e.IsDone {
				close(electedCh)
				return
			}
			electedCh <- e.IsLeader
		},
		Election:  campaign,
		Candidate: candidate,
		TTL:       1,
	}
	e := etcdutil.NewElectionAsync(s.proxiedClients[id], electionCfg)
	return e, electedCh
}
