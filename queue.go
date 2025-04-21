package gotsk

import (
	"context"
	"fmt"
	"sync"
)

type HandlerFunc func(context.Context, Payload) error

type Queue struct {
	mu         sync.RWMutex
	handlers   map[string]HandlerFunc
	workers    int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	store      TaskStore
	done       chan bool
	maxRetries int
}

func NewWithStore(workers int, store TaskStore) *Queue {
	ctx, cancel := context.WithCancel(context.Background())
	return &Queue{
		handlers:   make(map[string]HandlerFunc),
		workers:    workers,
		ctx:        ctx,
		cancel:     cancel,
		maxRetries: 3,
		store:      store,
		done:       make(chan bool, workers),
	}
}

func (q *Queue) Register(name string, handler HandlerFunc) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[name] = handler
}

func (q *Queue) Enqueue(name string, payload Payload) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if _, ok := q.handlers[name]; !ok {
		return fmt.Errorf("handler for task '%s' not registered", name)
	}
	return q.store.Push(Task{Name: name, Payload: payload})
}

func (q *Queue) Start() {
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker()
	}
}

func (q *Queue) Stop() {
	q.cancel()
	q.wg.Wait()

	select {
	case <-q.done:
	default:
		close(q.done)
	}
}
