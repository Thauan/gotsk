package gotsk

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Thauan/gotsk/interfaces"
)

// MemoryStore is an in-process TaskStore. Tasks are lost when the process exits.
// It honors TaskOptions.Priority: the highest-priority ready task is popped first.
type MemoryStore struct {
	mu      sync.Mutex
	queue   []interfaces.Task
	pending []interfaces.Task
}

// LenQueue returns the number of tasks waiting to be popped.
func (m *MemoryStore) LenQueue() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue)
}

// LenPending returns the number of popped tasks that have not been acknowledged.
func (m *MemoryStore) LenPending() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pending)
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		queue:   []interfaces.Task{},
		pending: []interfaces.Task{},
	}
}

// Push adds a task to the queue.
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

// Ack removes a popped task from the pending list, matched by ID.
func (s *MemoryStore) Ack(task interfaces.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, t := range s.pending {
		if t.ID == task.ID {
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
			return nil
		}
	}
	return errors.New("task not found in pending")
}

// Size devolve quantas tasks estão na fila (sem contar as pendentes).
func (s *MemoryStore) Size(_ context.Context) (int, error) {
	return s.LenQueue(), nil
}
