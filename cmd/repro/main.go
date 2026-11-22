// Programa de reprodução e de demonstração do painel de tarefas.
//
// Variáveis:
//   - UI_ADDR (padrão :8080): painel e /events, servidos por gotsk.Run.
//   - TRIGGER_ADDR (padrão :9090): POST /enqueue?n=N enfileira N tasks em rajada.
//   - POST /enqueue?n=N&fail=true cria tasks que falham.
//   - TASK_DURATION_MS: duração fixa de cada task em ms (padrão: aleatória de 0 a 30).
//   - HISTORY_DRIVER: sqlite (padrão), dynamodb ou none.
//   - SQLITE_PATH (padrão gotsk.db): arquivo do SQLite.
//   - DYNAMODB_TABLE (padrão gotsk_tasks), DYNAMODB_ENDPOINT (opcional, ex.: DynamoDB Local).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/Thauan/gotsk"
	"github.com/Thauan/gotsk/history/dynamodb"
	"github.com/Thauan/gotsk/history/sqlite"
	"github.com/Thauan/gotsk/interfaces"
	"github.com/Thauan/gotsk/store"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// openHistory escolhe o backend de histórico pela variável HISTORY_DRIVER.
func openHistory() (interfaces.HistoryRepository, error) {
	switch driver := env("HISTORY_DRIVER", "sqlite"); driver {
	case "none":
		return nil, nil
	case "sqlite":
		return sqlite.Open("file:" + env("SQLITE_PATH", "gotsk.db"))
	case "dynamodb":
		ctx := context.Background()
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(env("AWS_REGION", "us-east-1")))
		if err != nil {
			return nil, err
		}
		client := ddb.NewFromConfig(cfg, func(o *ddb.Options) {
			if endpoint := os.Getenv("DYNAMODB_ENDPOINT"); endpoint != "" {
				o.BaseEndpoint = aws.String(endpoint)
			}
		})
		table := env("DYNAMODB_TABLE", "gotsk_tasks")
		if err := dynamodb.EnsureTable(ctx, client, table); err != nil {
			return nil, err
		}
		return dynamodb.New(client, table), nil
	default:
		return nil, fmt.Errorf("HISTORY_DRIVER desconhecido: %q", driver)
	}
}

func main() {
	queue := gotsk.NewWithStore(4, store.NewMemoryStore())

	history, err := openHistory()
	if err != nil {
		log.Fatalf("erro ao abrir histórico: %v", err)
	}
	if history != nil {
		queue.SetHistory(history)
		log.Printf("histórico: %s", env("HISTORY_DRIVER", "sqlite"))
	}

	// TASK_DURATION_MS fixa a duração de cada task; sem ela, cada task dura de 0 a 30 ms.
	fixedMs, _ := strconv.Atoi(env("TASK_DURATION_MS", "0"))

	queue.Register("noop", func(ctx context.Context, payload interfaces.Payload) error {
		if payload["fail"] == true {
			return errors.New("falha simulada")
		}
		if fixedMs > 0 {
			time.Sleep(time.Duration(fixedMs) * time.Millisecond)
		} else {
			time.Sleep(time.Duration(rand.Intn(30)) * time.Millisecond)
		}
		return nil
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/enqueue", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "use POST", http.StatusMethodNotAllowed)
			return
		}
		n, err := strconv.Atoi(r.URL.Query().Get("n"))
		if err != nil || n <= 0 {
			http.Error(w, "parâmetro n inválido", http.StatusBadRequest)
			return
		}
		// fail=true cria tasks que falham (3 retentativas cada), para testar falhas.
		fail := r.URL.Query().Get("fail") == "true"
		for i := range n {
			if err := queue.Enqueue("noop", interfaces.Payload{"i": i, "fail": fail}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		fmt.Fprintf(w, "enfileiradas %d tasks\n", n)
	})

	triggerAddr := env("TRIGGER_ADDR", ":9090")
	go func() {
		log.Printf("gatilho de rajada em http://%s/enqueue?n=N", triggerAddr)
		log.Fatal(http.ListenAndServe(triggerAddr, mux))
	}()

	gotsk.Run(queue, env("UI_ADDR", ":8080"))
}
