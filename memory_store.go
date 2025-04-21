package gotsk

import (
	"errors"
	"sync"
)

type MemoryStore struct {
	mu      sync.Mutex
	queue   []Task
	pending []Task
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		queue:   []Task{},
		pending: []Task{},
	}
}

func (s *MemoryStore) Push(task Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = append(s.queue, task)
	return nil
}

func (s *MemoryStore) Pop() (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queue) == 0 {
		return Task{}, errors.New("no tasks available")
	}

	task := s.queue[0]
	s.queue = s.queue[1:]
	s.pending = append(s.pending, task)
	return task, nil
}

func (s *MemoryStore) Ack(task Task) error {
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

func equalPayload(a, b Payload) bool {
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
