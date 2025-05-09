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

```bash
go get github.com/Thauan/gotsk
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
store := gotsk.NewRedisStore("localhost:6379", "", 0, "gotsk:queue")
queue := gotsk.NewWithStore(4, store)
```

### 🛠️ SQS

```go
ctx := context.Background()

cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))

if err != nil {
	log.Fatalf("failed to load AWS config: %v", err)
}

client := sqs.NewFromConfig(cfg)

logger, err := zap.NewDevelopment()
if err != nil {
	log.Fatalf("Failed to initialize logger: %v", err)
}
defer logger.Sync()

store := interfaces.NewSQSStore(
	client,
	"https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
)

queue := gotsk.NewWithStore(4, store)
```

## Logging

### 🛠️ Standard Middleware

```go
logger := log.New(os.Stderr, "", log.LstdFlags)

queue := gotsk.NewWithStore(4, store.NewMemoryStore())
queue.Use(middlewares.LoggingMiddleware(logger))
```

### 🛠️ [uber-go/zap](https://github.com/uber-go/zap)

```go
logger, err := zap.NewDevelopment()
if err != nil {
	log.Fatalf("Failed to initialize logger: %v", err)
}

defer logger.Sync()

store := store.NewRedisStore("localhost:6379", "", 0, "gotsk:queue")
queue := gotsk.NewWithStore(4, store)
queue.Use(middlewares.ZapLoggingMiddleware(logger))
```

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
