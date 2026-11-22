// Programa de reprodução e de demonstração do painel de tarefas.
//
// Variáveis:
//   - UI_ADDR (padrão :8080): painel e /events, servidos por gotsk.Run.
//   - TRIGGER_ADDR (padrão :9090): POST /enqueue?n=N enfileira N tasks em rajada.
//   - POST /enqueue?n=N&fail=true cria tasks que falham.
//   - TASK_DURATION_MS: duração fixa de cada task em ms (padrão: aleatória de 0 a 30).
//   - HISTORY_DRIVER: sqlite (padrão), dynamodb ou none.
//   - SQLITE_PATH (padrão gotsk.db): arquivo do SQLite.
//   - QUEUE_DRIVER: memory (padrão) ou sqs.
//   - SQS_QUEUE_NAME (padrão gotsk-tasks): fila criada se não existir.
//   - SQS_DLQ_NAME (opcional): fila de mortos para tasks que esgotam as tentativas.
//   - SQS_ENDPOINT (opcional, ex.: LocalStack http://localstack:4566). Sem ele, usa a AWS real.
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
	"github.com/aws/aws-sdk-go-v2/credentials"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/Thauan/gotsk"
	"github.com/Thauan/gotsk/history/dynamodb"
	"github.com/Thauan/gotsk/history/sqlite"
	"github.com/Thauan/gotsk/interfaces"
	sqsstore "github.com/Thauan/gotsk/store/sqs"
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

// openQueueStore escolhe o store da fila pela variável QUEUE_DRIVER.
func openQueueStore() (interfaces.TaskStore, error) {
	switch driver := env("QUEUE_DRIVER", "memory"); driver {
	case "memory":
		return gotsk.NewMemoryStore(), nil
	case "sqs":
		return newSQSStore(env("SQS_QUEUE_NAME", "gotsk-tasks"))
	default:
		return nil, fmt.Errorf("QUEUE_DRIVER desconhecido: %q", driver)
	}
}

// newSQSStore cria (se preciso) a fila SQS com o nome dado e devolve o store.
func newSQSStore(name string) (*sqsstore.Store, error) {
	ctx := context.Background()
	opts := []func(*config.LoadOptions) error{config.WithRegion(env("AWS_REGION", "us-east-1"))}
	// Com endpoint local (LocalStack) usa credenciais fixas; na AWS real,
	// a cadeia padrão (variáveis, ~/.aws, role) é usada.
	if os.Getenv("SQS_ENDPOINT") != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			env("AWS_ACCESS_KEY_ID", "local"), env("AWS_SECRET_ACCESS_KEY", "local"), "")))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	client := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		if endpoint := os.Getenv("SQS_ENDPOINT"); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	// O SQS pode demorar a responder ao subir o container (ex.: LocalStack).
	var out *sqs.CreateQueueOutput
	for attempt := 1; ; attempt++ {
		out, err = client.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(name)})
		if err == nil {
			break
		}
		if attempt == 15 {
			return nil, fmt.Errorf("criando fila SQS %s: %w", name, err)
		}
		log.Printf("SQS indisponível (tentativa %d), tentando de novo...", attempt)
		time.Sleep(2 * time.Second)
	}
	log.Printf("fila SQS %s: %s", name, aws.ToString(out.QueueUrl))
	return sqsstore.New(client, aws.ToString(out.QueueUrl)), nil
}

func main() {
	store, err := openQueueStore()
	if err != nil {
		log.Fatalf("erro ao abrir a fila: %v", err)
	}
	queue := gotsk.NewWithStore(4, store)
	if dlqName := os.Getenv("SQS_DLQ_NAME"); dlqName != "" && env("QUEUE_DRIVER", "memory") == "sqs" {
		dlq, err := newSQSStore(dlqName)
		if err != nil {
			log.Fatalf("erro ao abrir a dead-letter: %v", err)
		}
		queue.SetDeadLetter(dlq)
	}

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
