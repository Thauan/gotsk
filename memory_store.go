package gotsk

import (
	"errors"
	"sync"
	"time"

	"github.com/Thauan/gotsk/interfaces"
)

type MemoryStore struct {
	mu      sync.Mutex
	queue   []interfaces.Task
	pending []interfaces.Task
}

func (m *MemoryStore) LenQueue() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue)
}

func (m *MemoryStore) LenPending() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pending)
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		queue:   []interfaces.Task{},
		pending: []interfaces.Task{},
	}
}

func (s *MemoryStore) Push(task interfaces.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = append(s.queue, task)
	return nil
}

// Pop devolve a task pronta de maior prioridade. Entre prioridades iguais,
// vale a ordem de chegada.
func (s *MemoryStore) Pop() (interfaces.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	best := -1
	for i, task := range s.queue {
		if task.ScheduledAt.After(now) {
			continue
		}
		if best == -1 || task.Priority > s.queue[best].Priority {
			best = i
		}
	}
	if best == -1 {
		return interfaces.Task{}, errors.New("no ready tasks available")
	}

	task := s.queue[best]
	s.queue = append(s.queue[:best], s.queue[best+1:]...)
	s.pending = append(s.pending, task)
	return task, nil
}

func (s *MemoryStore) Ack(task interfaces.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, t := range s.pending {
		if t.Name == task.Name && equalPayload(t.Payload, task.Payload) {
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
			return nil
		}
	}
	return errors.New("task not found in pending")
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
