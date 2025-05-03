package gotsk

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/gorilla/websocket"
)

type HandlerFunc interfaces.HandlerFunc

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

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
	history      []interfaces.Task
	sseClients   map[chan interfaces.Task]bool
	sseClientsMu sync.Mutex
}

var UIPath string

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

	time.Sleep(2 * time.Second)

	task := interfaces.Task{
		ID:        TaskId(),
		Name:      name,
		Payload:   payload,
		Status:    "queued",
		CreatedAt: time.Now(),
	}

	q.broadcast(task)

	return q.store.Push(task)
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

func (q *Queue) AddToHistory(task interfaces.Task) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.history = append(q.history, task)
}

func (q *Queue) GetHistory() []interfaces.Task {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.history
}

func (q *Queue) EnqueueAt(name string, payload interfaces.Payload, options interfaces.TaskOptions) error {
	time.Sleep(2 * time.Second)

	task := interfaces.Task{
		ID:          TaskId(),
		Name:        name,
		Payload:     payload,
		Status:      "scheduled",
		Priority:    options.Priority,
		ScheduledAt: options.ScheduledAt,
	}

	q.broadcast(task)

	return q.store.Push(task)
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

func (q *Queue) ServeUI(addr string, ctx context.Context) {
	go func() {
		if UIPath == "" {
			cwd, _ := os.Getwd()
			UIPath = filepath.Join(cwd, "web-ui", "dist")
		}

		log.Printf("🌐 Servindo arquivos estáticos de: %s\n", UIPath)

		fs := http.FileServer(http.Dir(UIPath))
		mux := http.NewServeMux()
		mux.Handle("/", fs)

		mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			ch := make(chan interfaces.Task)
			q.registerSSEClient(ch)
			defer q.unregisterSSEClient(ch)

			notify := r.Context().Done()

			for {
				select {
				case <-notify:
					return
				case task, ok := <-ch:
					if !ok {
						return
					}
					data, _ := json.Marshal(task)
					fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
				}
			}
		})

		srv := &http.Server{Addr: addr, Handler: mux}

		go func() {
			<-ctx.Done()
			log.Println("🛑 Encerrando servidor UI...")
			srv.Shutdown(context.Background())
		}()

		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Erro ao iniciar UI: %v", err)
		}
	}()
}

func (q *Queue) broadcast(task interfaces.Task) {
	q.sseClientsMu.Lock()
	defer q.sseClientsMu.Unlock()
	for ch := range q.sseClients {
		select {
		case ch <- task:
		default:
		}
	}
}
