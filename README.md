# 🐹 GoTSK - Task Queue Assíncrona em Go

**GoTSK** é uma fila de tarefas assíncrona leve e extensível escrita em Go. Inspirada no Sidekiq, ela permite registrar e executar tarefas de forma concorrente com suporte a diferentes backends de armazenamento, como memória ou Redis.

## ✨ Recursos

- Execução assíncrona com múltiplos workers
- Registro de handlers por nome
- Suporte a backends pluggáveis (`MemoryStore`, `RedisStore`)
- Retry automático com backoff exponencial
- Interface extensível para armazenamento (permite criar novos adapters)

---

## 🚀 Instalação

```bash
go get github.com/Thauan/gotsk
```

## Exemplos de uso
### 🧪 Uso com MemoryStore

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/Thauan/gotsk"
)

func main() {
	queue := gotsk.NewWithStore(4, gotsk.NewMemoryStore())

	queue.Register("send_email", func(ctx context.Context, payload gotsk.Payload) error {
		log.Println("Enviando email para:", payload["to"])
		return nil
	})

	queue.Start()
	defer queue.Stop()

	for i := 0; i < 5; i++ {
		queue.Enqueue("send_email", gotsk.Payload{
			"to":   "user@example.com",
			"body": "Olá, mundo!",
		})
	}

	time.Sleep(5 * time.Second)
}
```

### 🛠️ Uso com Redis

```go
store := gotsk.NewRedisStore("localhost:6379", "", 0, "gotsk:queue")
queue := gotsk.NewWithStore(4, store)
```


## ✅ Roadmap (ideias futuras)
- Suporte a tasks com atraso (delayed jobs)
- Deduplicação de tarefas
- Persistência em disco
- Web UI para monitoramento
- Middleware (log, métricas, tracing)

## 🤝 Contribuindo
Contribuições são bem-vindas! Sinta-se à vontade para abrir issues, enviar PRs ou sugerir melhorias.

📄 Licença
MIT License © Thauan Almeida