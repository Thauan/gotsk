package gotsk

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/Thauan/gotsk/middlewares"
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
	history      []interfaces.Task
	tasks        map[string]interfaces.Task
	queues       map[string][]string
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

	q.broadcast(task)

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
		Status:      "scheduled",
		Priority:    options.Priority,
		ScheduledAt: options.ScheduledAt,
	}

	if err := q.store.Push(task); err != nil {
		return err
	}

	q.broadcast(task)

	return nil
}

func (q *Queue) ListTasks() []interfaces.Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	tasks := make([]interfaces.Task, 0, len(q.tasks))
	for _, task := range q.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

func (q *Queue) ListQueues() map[string]int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make(map[string]int)
	for queueName, taskIDs := range q.queues {
		result[queueName] = len(taskIDs)
	}
	return result
}

func (q *Queue) GetStats() map[string]int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats := map[string]int{
		"total":    len(q.tasks),
		"queued":   0,
		"running":  0,
		"done":     0,
		"failed":   0,
		"canceled": 0,
	}

	for _, task := range q.tasks {
		switch task.Status {
		case "queued":
			stats["queued"]++
		case "running":
			stats["running"]++
		case "done":
			stats["done"]++
		case "failed":
			stats["failed"]++
		case "canceled":
			stats["canceled"]++
		}
	}

	return stats
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

func (q *Queue) streamTasks(w http.ResponseWriter, r *http.Request, tmpl *template.Template) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming não suportado", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	taskCh := q.Subscribe()
	defer q.unregisterSSEClient(taskCh)

	notify := r.Context().Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-notify:
			return
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case task := <-taskCh:
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, task); err != nil {
				http.Error(w, "Erro interno", http.StatusInternalServerError)
				log.Printf("Erro ao carregar template: %v", err)
				continue
			}

			eventName := "task-added"
			if task.Status != "queued" && task.Status != "scheduled" {
				eventName = "task-updated"
			}

			fmt.Fprintf(w, "event: %s\n", eventName)
			fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(buf.String(), "\n", ""))
			log.Printf("Enviando evento: %s para a tarefa: %s", eventName, task.ID)
			flusher.Flush()
		}
	}
}

func (q *Queue) GetProcessedTasks() []*interfaces.Task {
	var processed []*interfaces.Task
	q.mu.RLock()
	defer q.mu.RUnlock()
	for _, task := range q.tasks {
		if task.Status == "completed" || task.Status == "failed" {
			processed = append(processed, &task)
		}
	}
	return processed

}

func (q *Queue) handleGetProcessedTasks(w http.ResponseWriter) {
	tasks := q.GetProcessedTasks()
	fmt.Println(tasks)
	tmpl := template.Must(template.ParseFiles("web-ui/templates/partials/task-row.html"))

	var buf bytes.Buffer
	for _, task := range tasks {
		if err := tmpl.Execute(&buf, task); err != nil {
			log.Printf("Erro renderizando tarefa %s: %v", task.ID, err)
			continue
		}
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(buf.Bytes())
}

func (q *Queue) ServeUI(addr string, ctx context.Context) {
	go func() {
		mux := http.NewServeMux()

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			tmplPath := filepath.Join("web-ui", "templates", "task-dashboard.html")
			tmpl, err := template.ParseFiles(tmplPath)
			if err != nil {
				http.Error(w, "Erro ao carregar template", http.StatusInternalServerError)
				log.Println("Erro ao carregar template:", err)
				return
			}

			data := struct {
				Stats  any
				Queues any
				Tasks  any
			}{
				Stats:  q.GetStats(),
				Queues: q.ListQueues(),
				Tasks:  q.ListTasks(),
			}

			if err := tmpl.Execute(w, data); err != nil {
				http.Error(w, "Erro ao renderizar template", http.StatusInternalServerError)
				log.Println("Erro ao renderizar:", err)
			}
		})

		mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
			rowTmpl, err := template.ParseFiles(filepath.Join("web-ui", "templates", "partials", "task-row.html"))
			if err != nil {
				http.Error(w, "Erro ao carregar template", http.StatusInternalServerError)
				log.Println("Erro ao carregar template:", err)
				return
			}

			q.streamTasks(w, r, rowTmpl)
		})

		mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
			q.handleGetProcessedTasks(w)
		})

		srv := &http.Server{
			Addr:    addr,
			Handler: middlewares.HTTPLoggingMiddleware(log.New(os.Stdout, "", log.LstdFlags))(mux),
		}

		go func() {
			<-ctx.Done()
			log.Println("🛑 Encerrando servidor UI...")
			srv.Shutdown(context.Background())
		}()

		log.Printf("🌐 Servindo UI em http://%s", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Erro ao iniciar UI: %v", err)
		}
	}()
}

func (q *Queue) Subscribe() chan interfaces.Task {
	taskCh := make(chan interfaces.Task)

	q.registerSSEClient(taskCh)

	return taskCh
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
