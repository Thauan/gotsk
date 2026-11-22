package main

import (
	"context"
	"log"
	"time"

	"github.com/Thauan/gotsk"
	"github.com/Thauan/gotsk/interfaces"
	zapmw "github.com/Thauan/gotsk/middlewares/zap"
	redisstore "github.com/Thauan/gotsk/store/redis"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Erro ao inicializar o logger: %v", err)
	}
	defer logger.Sync()

	store := redisstore.New("localhost:6379", "", 0, "gotsk:queue")
	queue := gotsk.NewWithStore(4, store)
	queue.Use(zapmw.LoggingMiddleware(logger))

	queue.Register("send_email", func(ctx context.Context, payload interfaces.Payload) error {
		log.Println("Enviando email para:", payload["to"])
		return nil
	})

	queue.Start()
	defer queue.Stop()

	for range 5 {
		queue.Enqueue("send_email", interfaces.Payload{
			"to":   "user@example.com",
			"body": "Olá, mundo!",
		})
	}

	time.Sleep(5 * time.Second)
}
