package store

import (
	"sync"

	"github.com/Thauan/gotsk/interfaces"
)

type BaseStore struct {
	mu      sync.Mutex
	queue   []interfaces.Task
	pending []interfaces.Task
}

func (b *BaseStore) LenQueue() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.queue)
}

func (b *BaseStore) LenPending() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.pending)
}

func equalPayload(a, b interfaces.Payload) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
