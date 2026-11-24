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

A tela de **Configurações** permite pausar e retomar o processamento, reenfileirar falhas, devolver a dead-letter para a fila e limpar o histórico (por escopo, com confirmação). O idioma é escolhido pelas bandeiras no topo da página: português (padrão) ou inglês, guardado num cookie e, na primeira visita, seguindo o idioma do navegador.

## 💀 Dead-letter

Tasks que esgotam as tentativas podem ir para uma fila de mortos. Sem ela, a task é confirmada e removida da fila (o registro continua no histórico), em vez de ser reentregue para sempre.

```go
dlq := redisstore.New("localhost:6379", "", 0, "gotsk:dead")
queue.SetDeadLetter(dlq)

// Devolve as tasks da dead-letter para a fila principal.
moved, err := queue.RequeueDead(ctx)
```

A task só é confirmada na fila principal depois de gravada na dead-letter. Se a gravação falhar, ela é entregue de novo. Stores que contam as próprias mensagens (`interfaces.Sized`) mostram a contagem no painel.

## ⏸️ Pausa

`queue.Pause()` faz os workers pararem de pegar tasks novas. As que já estão rodando terminam, e `Enqueue` continua aceitando trabalho. `queue.Resume()` volta a processar. O estado da pausa não é persistido: reiniciar o processo volta a processar.

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
