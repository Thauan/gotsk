package gotsk

import (
	"context"
	"fmt"
	"sync"

	"github.com/Thauan/gotsk/interfaces"
)

type HandlerFunc interfaces.HandlerFunc

type Queue struct {
	mu          sync.RWMutex
	handlers    map[string]HandlerFunc
	workers     int
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	store       interfaces.TaskStore
	done        chan bool
	maxRetries  int
	middlewares []interfaces.Middleware
}

func (q *Queue) Use(mw interfaces.Middleware) {
	q.middlewares = append(q.middlewares, mw)
}

func (q *Queue) GetWorkers() int {
	return q.workers
}

func NewWithStore(workers int, store interfaces.TaskStore) *Queue {
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
	for i := len(q.middlewares) - 1; i >= 0; i-- {
		handler = HandlerFunc(q.middlewares[i](interfaces.HandlerFunc(handler)))
	}
	q.handlers[name] = handler
}

func (q *Queue) Enqueue(name string, payload interfaces.Payload) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if _, ok := q.handlers[name]; !ok {
		return fmt.Errorf("handler for task '%s' not registered", name)
	}

	return q.store.Push(interfaces.Task{
		ID:      TaskId(),
		Name:    name,
		Payload: payload,
	})
}

func (q *Queue) Start() {
	for range q.workers {
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

func (q *Queue) EnqueueAt(name string, payload interfaces.Payload, options interfaces.TaskOptions) error {
	task := interfaces.Task{
		ID:          TaskId(),
		Name:        name,
		Payload:     payload,
		Priority:    options.Priority,
		ScheduledAt: options.ScheduledAt,
	}

	return q.store.Push(task)
}
