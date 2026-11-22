package gotsk

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Thauan/gotsk/interfaces"
)

type HandlerFunc interfaces.HandlerFunc

type Queue struct {
	mu           sync.RWMutex
	handlers     map[string]HandlerFunc
	workers      int
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	store        interfaces.TaskStore
	done         chan bool
	maxRetries   int
	middlewares  []interfaces.Middleware
	repo         interfaces.HistoryRepository
	paused       atomic.Bool
	startedAt    atomic.Int64 // unix nano; 0 = não iniciada
	sseClients   map[chan interfaces.Task]bool
	sseClientsMu sync.Mutex
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

	task := interfaces.Task{
		ID:        TaskId(),
		Name:      name,
		Payload:   payload,
		Status:    "queued",
		CreatedAt: time.Now(),
	}

	if err := q.store.Push(task); err != nil {
		return err
	}

	q.publish(task)

	return nil
}

// func (q *Queue) Enqueue(name string, payload interfaces.Payload) error {
// 	q.mu.RLock()
// 	defer q.mu.RUnlock()
// 	if _, ok := q.handlers[name]; !ok {
// 		return fmt.Errorf("handler for task '%s' not registered", name)
// 	}

// 	time.Sleep(2 * time.Second)

// 	task := interfaces.Task{
// 		ID:        TaskId(),
// 		Name:      name,
// 		Payload:   payload,
// 		Status:    "queued",
// 		CreatedAt: time.Now(),
// 	}

// 	q.broadcast(task)

// 	return q.store.Push(task)
// }

// ErrNoHistory é retornado por operações que dependem do histórico.
var ErrNoHistory = errors.New("histórico não configurado")

// Pause faz os workers pararem de pegar tasks novas. As tasks em execução
// terminam normalmente, e Enqueue continua aceitando trabalho.
func (q *Queue) Pause() { q.paused.Store(true) }

// Resume volta a processar as tasks pendentes.
func (q *Queue) Resume() { q.paused.Store(false) }

func (q *Queue) Paused() bool { return q.paused.Load() }

// Uptime é o tempo desde Start. Zero se a fila não foi iniciada.
func (q *Queue) Uptime() time.Duration {
	started := q.startedAt.Load()
	if started == 0 {
		return 0
	}
	return time.Since(time.Unix(0, started))
}

// RequeueTask reenfileira uma falha específica e remove o registro antigo.
func (q *Queue) RequeueTask(ctx context.Context, id string) error {
	if q.repo == nil {
		return ErrNoHistory
	}
	t, err := q.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if t.Status != "failed" {
		return fmt.Errorf("task %s não está com falha (status: %s)", id, t.Status)
	}
	if err := q.Enqueue(t.Name, t.Payload); err != nil {
		return err
	}
	return q.repo.Delete(ctx, id)
}

// maxRequeue limita quantas falhas são reenfileiradas por chamada.
const maxRequeue = 10000

// RequeueFailed reenfileira as tasks com falha do histórico e remove o
// registro antigo. Tasks cujo handler não está mais registrado são puladas e
// continuam marcadas como falha.
func (q *Queue) RequeueFailed(ctx context.Context) (requeued, skipped int, err error) {
	if q.repo == nil {
		return 0, 0, ErrNoHistory
	}
	failed, err := q.repo.List(ctx, interfaces.HistoryFilter{Status: "failed", Limit: maxRequeue})
	if err != nil {
		return 0, 0, fmt.Errorf("listando falhas: %w", err)
	}
	for _, t := range failed {
		if err := q.Enqueue(t.Name, t.Payload); err != nil {
			skipped++
			continue
		}
		if err := q.repo.Delete(ctx, t.ID); err != nil {
			return requeued, skipped, fmt.Errorf("removendo falha antiga %s: %w", t.ID, err)
		}
		requeued++
	}
	return requeued, skipped, nil
}

func (q *Queue) Start() {
	q.startedAt.Store(time.Now().UnixNano())
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
		Status:      "scheduled",
		Priority:    options.Priority,
		ScheduledAt: options.ScheduledAt,
	}

	if err := q.store.Push(task); err != nil {
		return err
	}

	q.publish(task)

	return nil
}

func (q *Queue) registerSSEClient(ch chan interfaces.Task) {
	q.sseClientsMu.Lock()
	defer q.sseClientsMu.Unlock()
	if q.sseClients == nil {
		q.sseClients = make(map[chan interfaces.Task]bool)
	}
	q.sseClients[ch] = true
}

func (q *Queue) unregisterSSEClient(ch chan interfaces.Task) {
	q.sseClientsMu.Lock()
	defer q.sseClientsMu.Unlock()

	delete(q.sseClients, ch)
	close(ch)
}

// sseClientBuffer é o número de eventos que podem ficar pendentes para um
// cliente SSE. Sem buffer, qualquer rajada perde eventos enquanto o streamer
// está escrevendo na conexão.
const sseClientBuffer = 1024

func (q *Queue) Subscribe() chan interfaces.Task {
	taskCh := make(chan interfaces.Task, sseClientBuffer)

	q.registerSSEClient(taskCh)

	return taskCh
}

// SetHistory liga a persistência do histórico. Sem ele, a UI mostra só os
// eventos ao vivo e as contagens ficam zeradas.
func (q *Queue) SetHistory(repo interfaces.HistoryRepository) {
	q.repo = repo
}

// publish envia a mudança de estado para a UI em tempo real e, se houver
// histórico configurado, grava o estado atual da task.
func (q *Queue) publish(task interfaces.Task) {
	q.broadcast(task)

	if q.repo == nil {
		return
	}
	// Background: mesmo após Stop, os últimos estados precisam ser gravados.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := q.repo.Save(ctx, task); err != nil {
		log.Printf("erro ao salvar histórico da task %s: %v", task.ID, err)
	}
}

func (q *Queue) broadcast(task interfaces.Task) {
	q.sseClientsMu.Lock()
	defer q.sseClientsMu.Unlock()

	log.Printf("Broadcasting task: %s", task.ID)

	for ch := range q.sseClients {
		select {
		case ch <- task:
			log.Printf("Sent task %s to client", task.ID)
		default:
			log.Printf("Client channel is full for task %s", task.ID)
		}
	}
}
