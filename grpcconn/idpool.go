package grpcconn

import (
	"strconv"
	"sync"
)

// IDPool maintains a pool of ID values that can be released and reused.
// This is handy to keep cardinality low as gRPC connections are periodically
// released and reconnected and require an id.
// This solves the problem of infinitely incrementing ids used in Prometheus
// metric labels causing infinite growth of historical metric values.
type IDPool struct {
	pool      map[ID]bool
	allocated int
	allocMu   sync.RWMutex
}

type ID int

func NewIDPool() *IDPool {
	return &IDPool{
		pool: make(map[ID]bool),
	}
}

func (i *IDPool) Allocate() ID {
	i.allocMu.Lock()
	defer i.allocMu.Unlock()

	for id, allocFlag := range i.pool {
		if allocFlag {
			continue
		}

		i.allocated++
		i.pool[id] = true
		return id
	}

	// Dynamically expand pool.
	newID := ID(len(i.pool) + 1)
	i.pool[newID] = true
	return newID
}

func (i *IDPool) Release(id ID) {
	i.allocMu.Lock()
	defer i.allocMu.Unlock()

	if allocFlag, ok := i.pool[id]; !ok || !allocFlag {
		return
	}

	i.allocated--
	i.pool[id] = false
}

func (id ID) String() string {
	return strconv.Itoa(int(id))
}
