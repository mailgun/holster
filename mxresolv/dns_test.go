package mxresolv_test

import (
	"net"
	"sync"

	"github.com/foxcpp/go-mockdns"
)

type MockDNS struct {
	Server *mockdns.Server
	mu     sync.Mutex
}

func SpawnMockDNS(zones map[string]mockdns.Zone) (*MockDNS, error) {
	server, err := mockdns.NewServerWithLogger(zones, nullLogger{}, false)
	if err != nil {
		return nil, err
	}
	return &MockDNS{
		Server: server,
	}, nil
}

func (f *MockDNS) Stop() {
	_ = f.Server.Close()
}

func (f *MockDNS) Patch(r *net.Resolver) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Server.PatchNet(r)
}

func (f *MockDNS) UnPatch(r *net.Resolver) {
	f.mu.Lock()
	defer f.mu.Unlock()
	mockdns.UnpatchNet(r)
}

type nullLogger struct{}

func (l nullLogger) Printf(_ string, _ ...interface{}) {}
