📖 Read in [Portuguese](./README.md)

# Gotsk - Asynchronous Task Queue in Go

**Gotsk** is a lightweight and extensible asynchronous task queue written in Go. It allows you to register and execute tasks concurrently, with support for different storage backends like memory, SQS or Redis.

## ✨ Features

- Asynchronous execution with multiple workers using goroutines
- Handler registration by name
- Support for multiple task storage backends (`MemoryStore`, `RedisStore`, `SQSStore`)
- Logging support with standard middleware and integration with [uber-go/zap](https://github.com/uber-go/zap)
- Automatic retry with exponential backoff
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
