package gotsk

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Thauan/gotsk/interfaces"
)

// memHistory é um interfaces.HistoryRepository em memória para os testes do
// núcleo. Os adaptadores reais ficam em seus próprios módulos.
type memHistory struct {
	mu    sync.Mutex
	tasks map[string]interfaces.Task
}

// Name identifica o backend na tela de configurações.
func (m *memHistory) Name() string { return "memória (teste)" }

func newMemHistory() *memHistory {
	return &memHistory{tasks: make(map[string]interfaces.Task)}
}

func (m *memHistory) Save(_ context.Context, t interfaces.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks[t.ID] = t
	return nil
}

func (m *memHistory) Get(_ context.Context, id string) (interfaces.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return interfaces.Task{}, interfaces.ErrTaskNotFound
	}
	return t, nil
}

func (m *memHistory) matching(f interfaces.HistoryFilter) []interfaces.Task {
	var out []interfaces.Task
	for _, t := range m.tasks {
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		if f.Search != "" && !strings.Contains(t.ID, f.Search) && !strings.Contains(t.Name, f.Search) {
			continue
		}
		if !f.Since.IsZero() && t.CreatedAt.Before(f.Since) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (m *memHistory) List(_ context.Context, f interfaces.HistoryFilter) ([]interfaces.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.matching(f)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	limit := f.Limit
	if limit <= 0 {
		limit = interfaces.DefaultHistoryLimit
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *memHistory) Counts(_ context.Context, since time.Time) (interfaces.HistoryCounts, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var c interfaces.HistoryCounts
	for _, t := range m.matching(interfaces.HistoryFilter{Since: since}) {
		switch t.Status {
		case "queued":
			c.Queued++
		case "scheduled":
			c.Scheduled++
		case "running":
			c.Running++
		case "completed":
			c.Completed++
		case "failed":
			c.Failed++
		}
		if t.Retries > 0 {
			c.Retried++
		}
	}
	return c, nil
}

func (m *memHistory) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tasks, id)
	return nil
}

func (m *memHistory) Purge(_ context.Context, f interfaces.HistoryFilter) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	matches := m.matching(f)
	for _, t := range matches {
		delete(m.tasks, t.ID)
	}
	return len(matches), nil
}
