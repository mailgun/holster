//go:build holster_test_mode

package clock

import "sync"

var (
	providerMu sync.RWMutex
	provider   Clock = realtime
)

func setProvider(p Clock) {
	providerMu.Lock()
	provider = p
	providerMu.Unlock()
}

func getProvider() Clock {
	providerMu.RLock()
	p := provider
	providerMu.RUnlock()
	return p
}
