package gotsk

import (
	"errors"
	"sync"
)

type MemoryStore struct {
	tasks []Task
	mu    sync.Mutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) Push(task Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks = append(s.tasks, task)
	return nil
}

func (s *MemoryStore) Pop() (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.tasks) == 0 {
		return Task{}, errors.New("no tasks available")
	}

	task := s.tasks[0]
	s.tasks = s.tasks[1:]
	return task, nil
}

func (s *MemoryStore) Ack(task Task) error {
	return nil
}
