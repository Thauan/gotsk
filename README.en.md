📖 Read in [Portuguese](./README.md)

# Gotsk - Asynchronous Task Queue in Go

**Gotsk** is a lightweight and extensible asynchronous task queue written in Go. It allows you to register and execute tasks concurrently, with support for different storage backends like memory, SQS or Redis.

## ✨ Features

- Asynchronous execution with multiple workers using goroutines
- Handler registration by name
- Support for multiple task storage backends (`MemoryStore`, `RedisStore`, `SQSStore`)
- Logging support with standard middleware and integration with [uber-go/zap](https://github.com/uber-go/zap)
- Automatic retry with exponential backoff
- Dashboard in Portuguese and English, with a language selector (flags in the sidebar). The default follows the browser language.
- Per-task priority (`TaskOptions.Priority`), honored by the `MemoryStore`. Redis and SQS have no native priority and ignore it.
- Extensible interface for storage (allows creation of custom adapters)

---

## 🚀 Installation

The core has few dependencies. Adapters are separate modules: install only what you use.

```bash
go get github.com/Thauan/gotsk                   # core: queue, memory store and dashboard
go get github.com/Thauan/gotsk/store/redis       # Redis store
go get github.com/Thauan/gotsk/store/sqs         # SQS store
go get github.com/Thauan/gotsk/middlewares/zap   # zap logging
go get github.com/Thauan/gotsk/history/sqlite    # dashboard history in SQLite
go get github.com/Thauan/gotsk/history/dynamodb  # dashboard history in DynamoDB
```

## Usage Examples

### 🛠️ MemoryStore

```go
queue := gotsk.NewWithStore(4, gotsk.NewMemoryStore())

queue.Register("send_email", func(ctx context.Context, payload gotsk.Payload) error {
	log.Println("Sending email to:", payload["to"])
	return nil
})

queue.Start()
defer queue.Stop()

for range 5 {
	queue.EnqueueAt("send_email", interfaces.Payload{
		"to": "example@test.com",
	}, interfaces.TaskOptions{
		Priority:    1,
		ScheduledAt: time.Now().Add(1 * time.Minute),
	})
}

time.Sleep(5 * time.Second)
```

### 🛠️ Redis

```go
import redisstore "github.com/Thauan/gotsk/store/redis"

store := redisstore.New("localhost:6379", "", 0, "gotsk:queue")
queue := gotsk.NewWithStore(4, store)
```

### 🛠️ SQS

```go
import sqsstore "github.com/Thauan/gotsk/store/sqs"

store := sqsstore.New(client, "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue")
queue := gotsk.NewWithStore(4, store)
```

## Logging

### 🛠️ Standard Middleware

```go
logger := log.New(os.Stderr, "", log.LstdFlags)

queue := gotsk.NewWithStore(4, gotsk.NewMemoryStore())
queue.Use(middlewares.LoggingMiddleware(logger))
```

### 🛠️ [uber-go/zap](https://github.com/uber-go/zap)

```go
import zapmw "github.com/Thauan/gotsk/middlewares/zap"

logger, err := zap.NewDevelopment()
if err != nil {
	log.Fatalf("Error initializing logger: %v", err)
}
defer logger.Sync()

queue.Use(zapmw.LoggingMiddleware(logger))
```

## 🖥️ Web dashboard

The dashboard shows tasks, settings and the details of each execution. It is served by `gotsk.Run`. To see history, configure a repository:

```go
import "github.com/Thauan/gotsk/history/sqlite"

repo, err := sqlite.Open("file:gotsk.db")
if err != nil {
	log.Fatal(err)
}
queue.SetHistory(repo)

gotsk.Run(queue, ":8080") // dashboard at http://localhost:8080
```

Without `SetHistory` the queue works normally, and the dashboard only shows a notice that history is disabled.

The **Settings** page can pause and resume processing, requeue failures, move the dead-letter back to the queue, and clear history (by scope, with confirmation). The language is chosen with the flags at the top of the page: Portuguese (default) or English, stored in a cookie and, on the first visit, following the browser language.

## 💀 Dead-letter

Tasks that exhaust their retries can go to a dead-letter queue. Without one, the task is acknowledged and removed from the queue (the history keeps the record), instead of being redelivered forever.

```go
dlq := redisstore.New("localhost:6379", "", 0, "gotsk:dead")
queue.SetDeadLetter(dlq)

// Moves the dead-letter tasks back to the main queue.
moved, err := queue.RequeueDead(ctx)
```

A task is only acknowledged on the main queue after it is written to the dead-letter. If that write fails, the task is delivered again. Stores that can count their own messages (`interfaces.Sized`) show the count in the dashboard.

## ⏸️ Pause

`queue.Pause()` stops workers from picking up new tasks. Running tasks finish, and `Enqueue` keeps accepting work. `queue.Resume()` resumes processing. The pause state is not persisted: restarting the process resumes processing.

## ✅ Roadmap (future ideas)

- Delayed jobs
- Task deduplication
- Disk persistence
- Web UI for monitoring
- Middleware for metrics and tracing

## 🤝 Contributing

Contributions are welcome! Feel free to open issues, send PRs, or suggest improvements.

📄 License
MIT License © Thauan Almeida
