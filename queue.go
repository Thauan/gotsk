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
	history     []interfaces.Task
}

var UIPath string

func (q *Queue) ServeUI(addr string, ctx context.Context) {
	// cwd, err := os.Getwd()
	// if err != nil {
	// 	log.Fatalf("Erro ao obter diretório atual: %v", err)
	// }
	// uiPath := filepath.Join(cwd, "web-ui", "dist")

	// log.Printf("🌐 Servindo arquivos estáticos de: %s\n", uiPath)
	if UIPath == "" {
		cwd, _ := os.Getwd()
		UIPath = filepath.Join(cwd, "web-ui", "dist")
	}

	log.Printf("🌐 Servindo arquivos estáticos de: %s\n", UIPath)

	fs := http.FileServer(http.Dir(UIPath))
	mux := http.NewServeMux()
	mux.Handle("/", fs)

	mux.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		tasks := q.GetHistory()
		if tasks == nil {
			tasks = []interfaces.Task{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tasks)
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
	return q.store.Push(interfaces.Task{Name: name, Payload: payload})
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
