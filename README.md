📖 Read in [English](./README.en.md)
# Gotsk - Task Queue Assíncrona em Go

![image](https://github.com/user-attachments/assets/243eeab3-173d-4d61-8048-82dffdcc74c6)

**Gotsk** é uma fila de tarefas assíncrona leve e extensível escrita em Go. Ela permite registrar e executar tarefas de forma concorrente com suporte a diferentes backends de armazenamento, como memória, SQS ou Redis.

## ✨ Recursos

- Execução assíncrona com múltiplos workers utilizando goroutines
- Registro de handlers por nome
- Suporte a múltiplos mecanismos de armazenamento de tarefas (`MemoryStore`, `redis.Store`, `sqs.Store`)
- Suporte a logs com middleware padrão e integração com [uber-go/zap](https://github.com/uber-go/zap)
- Retry automático com backoff exponencial
- Painel em português e inglês, com seletor de idioma (bandeiras na barra lateral). O padrão segue o idioma do navegador.
- Prioridade por task (`TaskOptions.Priority`), respeitada pelo `MemoryStore`. Redis e SQS não têm prioridade nativa e a ignoram.
- Interface extensível para armazenamento (permite criar novos adapters)

---

## 🚀 Instalação

O núcleo tem poucas dependências. Os adaptadores são módulos separados: instale só o que você usa.

```bash
go get github.com/Thauan/gotsk                   # núcleo: fila, memória e painel
go get github.com/Thauan/gotsk/store/redis       # store Redis
go get github.com/Thauan/gotsk/store/sqs         # store SQS
go get github.com/Thauan/gotsk/middlewares/zap   # logging com zap
go get github.com/Thauan/gotsk/history/sqlite    # histórico do painel em SQLite
go get github.com/Thauan/gotsk/history/dynamodb  # histórico do painel em DynamoDB
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

### 🛠️ Middleware Padrão

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
	log.Fatalf("Erro ao inicializar o logger: %v", err)
}
defer logger.Sync()

queue.Use(zapmw.LoggingMiddleware(logger))
```

## 🖥️ Painel web

O painel mostra tarefas, configurações e o detalhe de cada execução. Ele é servido pelo próprio `gotsk.Run`. Para ver o histórico, configure um repositório:

```go
import "github.com/Thauan/gotsk/history/sqlite"

repo, err := sqlite.Open("file:gotsk.db")
if err != nil {
	log.Fatal(err)
}
queue.SetHistory(repo)

gotsk.Run(queue, ":8080") // painel em http://localhost:8080
```

Sem `SetHistory`, a fila funciona normalmente e o painel mostra só o aviso de histórico desativado.

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
