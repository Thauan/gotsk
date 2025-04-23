package handlers

import (
	"sync"
	"time"

	"github.com/Thauan/gotsk/interfaces"
)

type TaskExecution struct {
	Name      string
	Payload   interfaces.Payload
	StartTime time.Time
	EndTime   time.Time
	Error     string
}

type TaskHistory struct {
	mu    sync.RWMutex
	tasks []TaskExecution
}

func (h *TaskHistory) Add(exec TaskExecution) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tasks = append(h.tasks, exec)
}

func (h *TaskHistory) All() []TaskExecution {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]TaskExecution(nil), h.tasks...)
}
