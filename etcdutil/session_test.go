package etcdutil_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy"
	"github.com/mailgun/holster/v5/etcdutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	etcd "go.etcd.io/etcd/client/v3"
)

var proxy *toxiproxy.Proxy
var client *etcd.Client

func TestMain(m *testing.M) {
	proxy = toxiproxy.NewProxy()
	proxy.Name = "etcd"
	proxy.Upstream = "localhost:22379"
	proxy.Listen = "0.0.0.0:2379"

	if err := proxy.Start(); err != nil {
		fmt.Printf("failed to start toxiproxy: %s\n", err)
		os.Exit(1)
	}

	var err error
	client, err = etcdutil.NewClient(nil)
	if err != nil {
		fmt.Printf("failed to connect to etcd\n")
		os.Exit(1)
	}

	code := m.Run()
	proxy.Stop()
	client.Close()
	os.Exit(code)
}

func TestNewSession(t *testing.T) {
	leaseChan := make(chan etcd.LeaseID, 1)
	getLease := func() etcd.LeaseID {
		select {
		case id := <-leaseChan:
			return id
		case <-time.After(time.Second * 5):
			require.FailNow(t, "Timeout waiting for lease id")
		}
		return 0
	}

	session, err := etcdutil.NewSession(client, etcdutil.SessionConfig{
		Observer: func(leaseId etcd.LeaseID, err error) {
			if err != nil {
				t.Fatal(err)
			}
			leaseChan <- leaseId
		},
	})
	require.Nil(t, err)
	defer session.Close()

	assert.NotEqual(t, etcdutil.NoLease, getLease())

	session.Close()

	assert.Equal(t, etcdutil.NoLease, getLease())
}

func TestConnectivityLost(t *testing.T) {
	leaseChan := make(chan etcd.LeaseID, 5)

	getLease := func() etcd.LeaseID {
		select {
		case id := <-leaseChan:
			return id
		case <-time.After(time.Second * 5):
			require.FailNow(t, "Timeout waiting for lease id")
		}
		return 0
	}

	logrus.SetLevel(logrus.DebugLevel)

	session, err := etcdutil.NewSession(client, etcdutil.SessionConfig{
		Observer: func(leaseId etcd.LeaseID, err error) {
			if err != nil {
				t.Fatal(err)
			}
			leaseChan <- leaseId
		},
		TTL: 1,
	})
	require.Nil(t, err)
	defer session.Close()

	// Assert we get a valid lease id
	assert.NotEqual(t, etcdutil.NoLease, getLease())

	// Interrupt the connection
	proxy.Stop()

	// Wait for session to realize the connection is gone
	assert.Equal(t, etcdutil.NoLease, getLease())

	// Restore the connection
	require.Nil(t, proxy.Start())

	// We should get a new lease
	assert.NotEqual(t, etcdutil.NoLease, getLease())

	session.Close()

	// Should get a final NoLease after close
	assert.Equal(t, etcdutil.NoLease, getLease())
}
