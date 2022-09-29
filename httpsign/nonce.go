package httpsign

import (
	"sync"

	"github.com/mailgun/holster/v4/collections"
)

type nonceCache struct {
	sync.Mutex

	cache    *collections.TTLMap
	cacheTTL int
}

// Return a new nonceCache. Allows you to control cache capacity, ttl, as well as the TimeProvider.
func newNonceCache(capacity, cacheTTL int) (*nonceCache, error) {
	return &nonceCache{
		cache:    collections.NewTTLMap(capacity),
		cacheTTL: cacheTTL,
	}, nil
}

// inCache checks if a nonce is in the cache. If not, it adds it to the
// cache and returns false. Otherwise it returns true.
func (n *nonceCache) inCache(nonce string) (exists bool, err error) {
	n.Lock()
	defer n.Unlock()

	// check if the nonce is already in the cache
	_, exists = n.cache.Get(nonce)
	if exists {
		return
	}

	// it's not, so let's put it in the cache
	err = n.cache.Set(nonce, "", n.cacheTTL)
	return
}
