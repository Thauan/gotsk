package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Thauan/gotsk"
	"github.com/Thauan/gotsk/interfaces"
	"github.com/Thauan/gotsk/middlewares"
	"github.com/Thauan/gotsk/store"
)

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags)

	queue := gotsk.NewWithStore(4, store.NewMemoryStore())
	queue.Use(middlewares.LoggingMiddleware(logger))

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
