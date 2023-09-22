package grpcconn_test

import (
	"testing"

	"github.com/mailgun/holster/v4/grpcconn"
	"github.com/stretchr/testify/assert"
)

func TestIDPool(t *testing.T) {
	t.Run("Allocate and release", func(t *testing.T) {
		pool := grpcconn.NewIDPool()
		id := pool.Allocate()
		assert.Equal(t, 1, int(id))
		pool.Release(id)
	})

	t.Run("Redundant release id ignored", func(t *testing.T) {
		pool := grpcconn.NewIDPool()
		id := pool.Allocate()
		pool.Release(id)
		pool.Release(id)
	})

	t.Run("Allocate and release all", func(t *testing.T) {
		const poolSize = 10
		pool := grpcconn.NewIDPool()
		var ids []grpcconn.ID

		// Allocate all.
		for i := 0; i < poolSize; i++ {
			id := pool.Allocate()
			assert.Equal(t, i+1, int(id))
			ids = append(ids, id)
		}

		// Release all.
		for _, id := range ids {
			pool.Release(id)
		}

		// Allocate all again.
		for i := 0; i < poolSize; i++ {
			id := pool.Allocate()
			// Expect id to reuse original pool of ids.
			assert.LessOrEqual(t, int(id), poolSize)
		}
	})
}
