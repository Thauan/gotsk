package store

import (
	"errors"
	"time"

	"slices"

	"github.com/Thauan/gotsk/interfaces"
)

type MemoryStore struct {
	BaseStore
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		BaseStore: BaseStore{
			queue:   []interfaces.Task{},
			pending: []interfaces.Task{},
		},
	}
}

func (s *MemoryStore) Push(task interfaces.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = append(s.queue, task)
	return nil
}

func (s *MemoryStore) Pop() (interfaces.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	for i, task := range s.queue {
		if task.ScheduledAt.IsZero() || !task.ScheduledAt.After(now) {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			s.pending = append(s.pending, task)
			return task, nil
		}
	}

	return interfaces.Task{}, errors.New("no task ready")
}

func (s *MemoryStore) Ack(task interfaces.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, t := range s.pending {
		if t.Name == task.Name && equalPayload(t.Payload, task.Payload) {
			s.pending = append(s.pending[:i], s.pending[i+1:]...)

			s.tasks = append(s.tasks, task)
			return nil
		}
	}
	return errors.New("task not found in pending")
}

func (s *MemoryStore) Tasks() []interfaces.Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.tasks)
}
