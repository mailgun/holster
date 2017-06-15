package httpsign

import (
	"sync"

	"github.com/mailgun/holster"
)

type NonceCache struct {
	sync.Mutex
	cache    *holster.TTLMap
	cacheTTL int
	clock    holster.Clock
}

// Return a new NonceCache. Allows you to control cache capacity and ttl
func NewNonceCache(capacity int, cacheTTL int, clock holster.Clock) *NonceCache {
	return &NonceCache{
		cache:    holster.NewTTLMapWithClock(capacity, clock),
		cacheTTL: cacheTTL,
		clock:    clock,
	}
}

// InCache checks if a nonce is in the cache. If not, it adds it to the
// cache and returns false. Otherwise it returns true.
func (n *NonceCache) InCache(nonce string) bool {
	n.Lock()
	defer n.Unlock()

	// check if the nonce is already in the cache
	_, exists := n.cache.Get(nonce)
	if exists {
		return true
	}

	// it's not, so let's put it in the cache
	n.cache.Set(nonce, "", n.cacheTTL)

	return false
}
