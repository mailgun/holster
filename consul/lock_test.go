//nolint:gocritic // ignore sloppyTestFuncName
package consul_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy"
	"github.com/hashicorp/consul/api"
	"github.com/mailgun/holster/v4/consul"
	"github.com/mailgun/holster/v4/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func cleanUp(t *testing.T) {
	client, _ := api.NewClient(api.DefaultConfig())
	list, _, _ := client.KV().List("lock-test", nil)
	for _, pair := range list {
		_, err := client.KV().Delete(pair.Key, nil)
		require.NoError(t, err)
	}
}

func WithToxiProxy(t *testing.T, fn func(*toxiproxy.Proxy, *api.Client)) {
	proxy := toxiproxy.NewProxy()
	proxy.Name = fmt.Sprintf("consul-proxy-%d", rand.Int())
	proxy.Listen = "127.0.0.1:0"
	proxy.Upstream = "127.0.0.1:8500"

	err := proxy.Start()
	defer proxy.Stop()
	require.NoError(t, err)

	cfg := api.DefaultConfig()
	cfg.Address = proxy.Listen
	c, err := api.NewClient(cfg)
	require.NoError(t, err)
	fn(proxy, c)
}

func printKeys(prefix string) {
	out := "-----------------\n"
	client, _ := api.NewClient(api.DefaultConfig())
	list, _, _ := client.KV().List(prefix, nil)
	for _, pair := range list {
		out += fmt.Sprintf("Pair: %s:%s\n", pair.Key, string(pair.Value))
	}
	fmt.Print(out + "-----\n")
}

func TestBehaviorRelease(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	hasLockCh := make(chan bool, 5)
	defer cleanUp(t)

	WithToxiProxy(t, func(p *toxiproxy.Proxy, c *api.Client) {
		printKeys("lock-test")
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
		defer cancel()
		name := fmt.Sprintf("lock-test-%d", rand.Int())
		l, err := consul.SpawnLock(ctx, &consul.LockConfig{
			Client: c,
			LockOptions: &api.LockOptions{
				SessionOpts: &api.SessionEntry{
					Name:      name,
					LockDelay: time.Second * 10,
					Behavior:  api.SessionBehaviorRelease,
					TTL:       "10s",
				},
				Value:       []byte("lock-value"),
				SessionName: name,
				Key:         name,
			},
			OnChange: func(hasLock bool) {
				hasLockCh <- hasLock
			},
		})
		require.NoError(t, err)
		require.NotNil(t, l)

		// Should have lock
		printKeys("lock-test")
		require.True(t, l.HasLock())
		select {
		case is := <-hasLockCh:
			assert.True(t, is)
		default:
		}

		// Interrupt connectivity to consul
		p.Stop()

		// Ensure the connectivity is broken
		_, _, err = c.KV().Get(name, nil)
		assert.Error(t, err)

		// Wait until we notice we lost lock
		is := <-hasLockCh
		assert.False(t, is)
		assert.False(t, l.HasLock())
		printKeys("lock-test")

		// Resume connectivity
		err = p.Start()
		require.NoError(t, err)

		// Wait until we regain lock
		is = <-hasLockCh
		assert.True(t, is)
		assert.True(t, l.HasLock())

		// Unlock the key
		l.Unlock([]byte("this is a test"))
		printKeys("lock-test")

		// Ensure the key is NOT deleted
		list, _, err := c.KV().List("lock-test", nil)
		require.NoError(t, err)
		var found bool
		for _, i := range list {
			if i.Key == name {
				found = true
				assert.Equal(t, i.Value, []byte("this is a test"))
			}
		}
		assert.True(t, found)

		// Delete the key
		_, err = c.KV().Delete(name, nil)
		require.NoError(t, err)
	})
}

func TestBehaviorDeleteOnUnlock(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	hasLockCh := make(chan bool, 5)
	defer cleanUp(t)

	WithToxiProxy(t, func(p *toxiproxy.Proxy, c *api.Client) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
		defer cancel()
		name := fmt.Sprintf("lock-test-delete-%d", rand.Int())
		l, err := consul.SpawnLock(ctx, &consul.LockConfig{
			Client: c,
			LockOptions: &api.LockOptions{
				SessionOpts: &api.SessionEntry{
					Name:      name,
					LockDelay: time.Second * 10,
					// When SessionBehavior is Delete
					Behavior: api.SessionBehaviorDelete,
					TTL:      "10s",
				},
				Value:       []byte("lock-value"),
				SessionName: name,
				Key:         name,
			},
			OnChange: func(hasLock bool) {
				hasLockCh <- hasLock
			},
		})
		require.NoError(t, err)
		require.NotNil(t, l)

		// Should have lock
		require.True(t, l.HasLock())
		select {
		case is := <-hasLockCh:
			assert.True(t, is)
		default:
		}

		// Unlock the key
		l.Unlock(nil)
		printKeys("lock-test")

		// Ensure the key was deleted
		list, _, err := c.KV().List("lock-test", nil)
		require.NoError(t, err)
		var found bool
		for _, i := range list {
			if i.Key == name {
				found = true
			}
		}
		assert.False(t, found)
	})
}

func TestBehaviorDeleteOnDisconnect(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	defer cleanUp(t)

	WithToxiProxy(t, func(p *toxiproxy.Proxy, c *api.Client) {
		printKeys("lock-test")
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
		defer cancel()
		name := fmt.Sprintf("lock-test-disconnect%d", rand.Int())
		l, err := consul.SpawnLock(ctx, &consul.LockConfig{
			Client: c,
			LockOptions: &api.LockOptions{
				SessionOpts: &api.SessionEntry{
					Name: name,
					// When SessionBehavior is Delete
					Behavior: api.SessionBehaviorDelete,
					TTL:      "10s",
				},
				Value:       []byte("lock-value"),
				SessionName: name,
				Key:         name,
			},
		})
		require.NoError(t, err)
		require.NotNil(t, l)

		// Should have lock
		printKeys("lock-test")
		require.True(t, l.HasLock())

		// Interrupt connectivity to consul
		p.Stop()

		// Wait for the lock file to disappear
		client, err := api.NewClient(api.DefaultConfig())
		require.NoError(t, err)
		testutil.UntilPass(t, 50, time.Second, func(t testutil.TestingT) {
			kv, _, err := client.KV().Get(name, nil)
			assert.NoError(t, err)
			assert.Nil(t, kv)
		})

		// Resume connectivity
		err = p.Start()
		require.NoError(t, err)

		// Wait until we regain lock
		testutil.UntilPass(t, 50, time.Second, func(t testutil.TestingT) {
			assert.True(t, l.HasLock())
		})

		// Unlock the key
		l.Unlock(nil)
	})
}
