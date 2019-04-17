package etcdutil_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy"
	etcd "github.com/coreos/etcd/clientv3"
	"github.com/mailgun/holster/etcdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var proxy *toxiproxy.Proxy
var client *etcd.Client

func TestMain(m *testing.M) {
	proxy = toxiproxy.NewProxy()
	proxy.Name = "etcd"
	proxy.Upstream = "localhost:22379"
	proxy.Listen = "0.0.0.0:2379"

	if err := proxy.Start(); err != nil {
		fmt.Printf("failed to start toxiproxy\n")
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := etcdutil.NewSession(ctx, client, etcdutil.SessionConfig{
		Observer: func(leaseId etcd.LeaseID) {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//logrus.SetLevel(logrus.DebugLevel)

	session, err := etcdutil.NewSession(ctx, client, etcdutil.SessionConfig{
		//Log: logrus.WithField("category", "test"),
		Observer: func(leaseId etcd.LeaseID) {
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
