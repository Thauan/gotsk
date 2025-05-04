📖 Read in [English](./README.en.md)
# Gotsk - Task Queue Assíncrona em Go

![image](https://github.com/user-attachments/assets/243eeab3-173d-4d61-8048-82dffdcc74c6)

**Gotsk** é uma fila de tarefas assíncrona leve e extensível escrita em Go. Ela permite registrar e executar tarefas de forma concorrente com suporte a diferentes backends de armazenamento, como memória, SQS ou Redis.

## ✨ Recursos

- Execução assíncrona com múltiplos workers utilizando goroutines
- Registro de handlers por nome
- Suporte a múltiplos mecanismos de armazenamento de tarefas (`MemoryStore`, `RedisStore`, `SQSStore`)
- Suporte a logs com middleware padrão e integração com [uber-go/zap](https://github.com/uber-go/zap)
- Retry automático com backoff exponencial
- Interface extensível para armazenamento (permite criar novos adapters)

---

## 🚀 Instalação

```bash
go get github.com/Thauan/gotsk
```

## Exemplos de uso

### 🛠️ MemoryStore

```go
queue := gotsk.NewWithStore(4, gotsk.NewMemoryStore())

queue.Register("send_email", func(ctx context.Context, payload gotsk.Payload) error {
	log.Println("Enviando email para:", payload["to"])
	return nil
})

queue.Start()
defer queue.Stop()

for range 5 {
	queue.EnqueueAt("send_email", interfaces.Payload{
		"to": "exemplo@teste.com",
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
	log.Fatalf("Erro ao inicializar o logger: %v", err)
}
defer logger.Sync()

store := interfaces.NewSQSStore(
	client,
	"https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
)

queue := gotsk.NewWithStore(4, store)
```

## Logging

### 🛠️ Middleware Padrão

```go
logger := log.New(os.Stderr, "", log.LstdFlags)

queue := gotsk.NewWithStore(4, store.NewMemoryStore())
queue.Use(middlewares.LoggingMiddleware(logger))
```

### 🛠️ [uber-go/zap](https://github.com/uber-go/zap)

```go
logger, err := zap.NewDevelopment()
if err != nil {
	log.Fatalf("Erro ao inicializar o logger: %v", err)
}

defer logger.Sync()

store := store.NewRedisStore("localhost:6379", "", 0, "gotsk:queue")
queue := gotsk.NewWithStore(4, store)
queue.Use(middlewares.ZapLoggingMiddleware(logger))
```

## ✅ Roadmap (ideias futuras)

- Suporte a tasks com atraso (delayed jobs)
- Deduplicação de tarefas
- Persistência em disco
- Web UI para monitoramento
- Middleware (métricas e tracing)

## 🤝 Contribuindo

Contribuições são bem-vindas! Sinta-se à vontade para abrir issues, enviar PRs ou sugerir melhorias.

📄 Licença
MIT License © Thauan Almeida
