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
	task := interfaces.Task{
		ID:          TaskId(),
		Name:        name,
		Payload:     payload,
		Status:      "scheduled",
		Priority:    options.Priority,
		ScheduledAt: options.ScheduledAt,
	}

	q.broadcast(task)

	time.Sleep(2 * time.Second)

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
				Stats:  nil,
				Queues: nil,
				Tasks:  nil,
			}

			if err := tmpl.Execute(w, data); err != nil {
				http.Error(w, "Erro ao renderizar template", http.StatusInternalServerError)
				log.Println("Erro ao renderizar:", err)
			}
		})

		var rowTmpl = template.Must(template.New("row").Parse(`
			<tr id="task-{{.ID}}" data-task-id="{{.ID}}" class="border-b transition-colors hover:bg-muted/50 data-[state=selected]:bg-muted">
				<td class="px-4 py-2">{{.Status}}</td>
				<td class="px-4 py-2">{{.ID}} / {{.Name}}</td>
				<td class="px-4 py-2 text-right">ações</td>
			</tr>
		`))

		mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
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
				case task := <-ch:
					var buf bytes.Buffer
					if err := rowTmpl.Execute(&buf, task); err != nil {
						log.Println("template error:", err)
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
