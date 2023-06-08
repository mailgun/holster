package discovery_test

import (
	"io"
	"log"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/foxcpp/go-mockdns"
	"github.com/mailgun/holster/v4/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/resolver"
)

type testClientConn struct {
	resolver.ClientConn // For unimplemented functions
	target              string
	m1                  sync.Mutex
	state               resolver.State
	updateStateCalls    int
	errChan             chan error
	updateChan          chan struct{}
	updateStateErr      error
}

func (t *testClientConn) UpdateState(s resolver.State) error {
	t.m1.Lock()
	defer t.m1.Unlock()
	t.state = s
	t.updateStateCalls++
	t.updateChan <- struct{}{}
	return t.updateStateErr
}

func (t *testClientConn) ReportError(err error) {
	t.errChan <- err
}

func TestSrvResolverBuilderSuccess(t *testing.T) {
	t.Skip("TODO: fix https://github.com/mailgun/holster/issues/150")

	z := map[string]mockdns.Zone{
		"srv.example.com.": {
			SRV: []net.SRV{
				{
					Target: "one.example.com.",
					Port:   12345,
				},
			},
			A: []string{"192.168.1.1"},
		},
		"one.example.com.": {
			A: []string{"192.168.1.1", "10.2.1.100"},
		},
	}
	dns, err := mockdns.NewServerWithLogger(z, log.New(io.Discard, "", log.LstdFlags), false)
	require.NoError(t, err)
	dns.PatchNet(net.DefaultResolver)
	defer func() {
		mockdns.UnpatchNet(net.DefaultResolver)
		dns.Close()
	}()

	b := discovery.NewGRPCSRVBuilder()
	cc := &testClientConn{target: "srv.example.com", updateChan: make(chan struct{}, 1), errChan: make(chan error, 10)}
	target := resolver.Target{URL: url.URL{Path: "srv.example.com:4567"}}
	r, err := b.Build(target, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	defer r.Close()

	// This request should be ignored by resolver, since we attempt to
	// resolve SRV records as soon as Build() is called.
	r.ResolveNow(resolver.ResolveNowOptions{})

	// UpdateState should have been called once
	select {
	case <-cc.updateChan:
	case <-time.After(time.Second * 3):
		require.FailNow(t, "timeout waiting for UpdateState() call")
	}

	assert.Equal(t, "192.168.1.1:12345", cc.state.Addresses[0].Addr)
	assert.Equal(t, "10.2.1.100:12345", cc.state.Addresses[1].Addr)
}

func TestSrvResolverBuilderNoARecord(t *testing.T) {
	z := map[string]mockdns.Zone{
		"srv.example.com.": {
			SRV: []net.SRV{
				{
					Target: "one.example.com.",
					Port:   12345,
				},
			},
		},
	}
	dns, err := mockdns.NewServerWithLogger(z, log.New(io.Discard, "", log.LstdFlags), false)
	require.NoError(t, err)
	dns.PatchNet(net.DefaultResolver)
	defer func() {
		mockdns.UnpatchNet(net.DefaultResolver)
		dns.Close()
	}()

	b := discovery.NewGRPCSRVBuilder()
	cc := &testClientConn{target: "srv.example.com", updateChan: make(chan struct{}, 1), errChan: make(chan error, 10)}
	target := resolver.Target{URL: url.URL{Path: "srv.example.com"}}
	r, err := b.Build(target, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	defer r.Close()

	// This request should be ignored, since we attempt to
	// resolve SRV records as soon as Build() is called.
	r.ResolveNow(resolver.ResolveNowOptions{})

	err = <-cc.errChan
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SRV record for 'srv.example.com' contained no valid domain names")

	// Should retry with back off
	err = <-cc.errChan
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SRV record for 'srv.example.com' contained no valid domain names")
}

func TestSrvResolverBuilderNoSRVRecord(t *testing.T) {
	z := map[string]mockdns.Zone{
		"srv.example.com.": {
			A: []string{"10.5.4.3"},
		},
	}
	dns, err := mockdns.NewServerWithLogger(z, log.New(io.Discard, "", log.LstdFlags), false)
	require.NoError(t, err)
	dns.PatchNet(net.DefaultResolver)
	defer func() {
		mockdns.UnpatchNet(net.DefaultResolver)
		dns.Close()
	}()

	b := discovery.NewGRPCSRVBuilder()
	cc := &testClientConn{target: "srv.example.com", updateChan: make(chan struct{}, 1), errChan: make(chan error, 10)}

	// SRV lookup will ignore the port number here, only if SRV lookup fails will this port number matter
	target := resolver.Target{URL: url.URL{Path: "srv.example.com:12345"}}
	r, err := b.Build(target, cc, resolver.BuildOptions{})
	require.NoError(t, err)
	defer r.Close()

	// This request should be ignored, since we attempt to
	// resolve SRV records as soon as Build() is called.
	r.ResolveNow(resolver.ResolveNowOptions{})

	err = <-cc.errChan
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SRV record lookup err: lookup srv.example.com on")
}
