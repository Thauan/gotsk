package main

import (
	"context"
	"log"
	"time"

	"github.com/Thauan/gotsk"
	"github.com/Thauan/gotsk/interfaces"
	"github.com/Thauan/gotsk/middlewares"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.uber.org/zap"
)

func main() {
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
	queue.Use(middlewares.ZapLoggingMiddleware(logger))

	queue.Register("send_email", func(ctx context.Context, payload interfaces.Payload) error {
		log.Println("Enviando email para:", payload["to"])
		return nil
	})

	queue.Start()
	defer queue.Stop()

	for range 5 {
		err := queue.Enqueue("send_email", interfaces.Payload{
			"to":   "user@example.com",
			"body": "Olá, mundo!",
		})
		if err != nil {
			log.Fatalf("erro ao enfileirar a tarefa: %v", err)
		}
	}

	time.Sleep(5 * time.Second)
}
