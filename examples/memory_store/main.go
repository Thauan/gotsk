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

	for range 5 {
		queue.EnqueueAt("send_email", interfaces.Payload{
			"to": "exemplo@teste.com",
		}, interfaces.TaskOptions{
			Priority:    1,
			ScheduledAt: time.Now().Add(30 * time.Second),
		})
	}

	gotsk.Run(queue, ":8080")
}
