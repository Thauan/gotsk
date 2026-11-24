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

// HandlerFunc is the function that processes a task.
type HandlerFunc interfaces.HandlerFunc

// Queue runs tasks on a fixed pool of workers, backed by a TaskStore.
// Create one with NewWithStore, register handlers, then call Start.
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
	dlq          interfaces.TaskStore
	uiToken      string
	uiSession    string
	paused       atomic.Bool
	startedAt    atomic.Int64 // unix nano; 0 = não iniciada
	sseClients   map[chan interfaces.Task]bool
	sseClientsMu sync.Mutex
}

// Use adds a middleware. Middlewares run in the order they were added,
// and apply to every task processed after the call.
func (q *Queue) Use(mw interfaces.Middleware) {
	q.middlewares = append(q.middlewares, mw)
}

// GetWorkers returns the number of workers the queue was created with.
func (q *Queue) GetWorkers() int {
	return q.workers
}

// NewWithStore creates a queue with the given number of workers and store.
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

// Register associa um handler ao nome. Os middlewares são aplicados na
// execução (process), então valem também para middlewares adicionados
// depois do registro, e são aplicados uma única vez por task.
func (q *Queue) Register(name string, handler HandlerFunc) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[name] = handler
}

// Enqueue adds a task for the named handler. It fails if no handler
// is registered under name.
func (q *Queue) Enqueue(name string, payload interfaces.Payload) error {
	// O lock protege só a leitura do mapa de handlers. A escrita no store e
	// no histórico acontece sem ele, para não bloquear Register.
	q.mu.RLock()
	_, ok := q.handlers[name]
	q.mu.RUnlock()
	if !ok {
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

// ErrNoDeadLetter é retornado por operações que dependem de SetDeadLetter.
var ErrNoDeadLetter = errors.New("dead-letter not configured")

// SetDeadLetter define a fila de mortos: onde vão as tasks que esgotam as
// tentativas. Ela pode ser outro store (ex.: uma segunda fila SQS).
func (q *Queue) SetDeadLetter(dlq interfaces.TaskStore) {
	q.dlq = dlq
}

// DeadLetterConfigured diz se há uma dead-letter definida.
func (q *Queue) DeadLetterConfigured() bool { return q.dlq != nil }

// DeadLetterSize devolve quantas tasks há na dead-letter. ok é false quando o
// store da dead-letter não sabe contar (não implementa interfaces.Sized).
func (q *Queue) DeadLetterSize(ctx context.Context) (n int, ok bool, err error) {
	sized, isSized := q.dlq.(interfaces.Sized)
	if !isSized {
		return 0, false, nil
	}
	n, err = sized.Size(ctx)
	return n, err == nil, err
}

// RequeueDead devolve tasks da dead-letter para a fila principal, até o
// limite de maxRequeue. Devolve quantas foram movidas.
func (q *Queue) RequeueDead(ctx context.Context) (int, error) {
	if q.dlq == nil {
		return 0, ErrNoDeadLetter
	}
	// Não usa a contagem do store como limite: ela pode estar atrasada (no SQS
	// é aproximada). Para quando o store fica vazio, quando uma task volta a
	// aparecer nesta chamada (ela falhou de novo e foi para a dead-letter) ou
	// ao atingir maxRequeue.
	limit := maxRequeue
	seen := make(map[string]bool)
	moved := 0
	for moved < limit {
		if err := ctx.Err(); err != nil {
			return moved, err
		}
		task, err := q.dlq.Pop()
		if err != nil {
			// Store vazio (ou indisponível): para aqui. Quem chama vê a contagem.
			return moved, nil
		}
		if seen[task.ID] {
			return moved, nil
		}
		seen[task.ID] = true
		if err := q.store.Push(task); err != nil {
			return moved, fmt.Errorf("devolvendo task %s: %w", task.ID, err)
		}
		if err := q.dlq.Ack(task); err != nil {
			log.Printf("error acknowledging task %s in the dead-letter: %v", task.ID, err)
		}
		task.Status = "queued"
		q.publish(task)
		moved++
	}
	return moved, nil
}

// ErrNoHistory é retornado por operações que dependem do histórico.
var ErrNoHistory = errors.New("history not configured")

// ErrTaskNotFailed é retornado por RequeueTask quando a task não está com falha.
var ErrTaskNotFailed = errors.New("task is not failed")

// Pause faz os workers pararem de pegar tasks novas. As tasks em execução
// terminam normalmente, e Enqueue continua aceitando trabalho.
func (q *Queue) Pause() { q.paused.Store(true) }

// Resume volta a processar as tasks pendentes.
func (q *Queue) Resume() { q.paused.Store(false) }

// Paused reports whether processing is paused.
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
		return fmt.Errorf("task %s: %w (status: %s)", id, ErrTaskNotFailed, t.Status)
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

// Start launches the workers. Call it once, after registering handlers.
func (q *Queue) Start() {
	q.startedAt.Store(time.Now().UnixNano())
	for range q.workers {
		q.wg.Add(1)
		go q.worker()
	}
}

// Stop cancels the queue context and waits for the workers to finish their
// current task. Tasks waiting in a retry backoff are not waited for.
func (q *Queue) Stop() {
	q.cancel()
	q.wg.Wait()

	select {
	case <-q.done:
	default:
		close(q.done)
	}
}

// EnqueueAt adds a task with options: priority and, optionally, a time
// before which it must not run.
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

// Subscribe returns a channel that receives every task state change.
// The channel is buffered; events are dropped if the reader falls behind.
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
		log.Printf("error saving history for task %s: %v", task.ID, err)
	}
}

func (q *Queue) broadcast(task interfaces.Task) {
	q.sseClientsMu.Lock()
	defer q.sseClientsMu.Unlock()

	for ch := range q.sseClients {
		select {
		case ch <- task:
		default:
			log.Printf("SSE client buffer full, dropping event for task %s", task.ID)
		}
	}
}
