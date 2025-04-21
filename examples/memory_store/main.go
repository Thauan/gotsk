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

	for range 5 {
		queue.Enqueue("send_email", gotsk.Payload{
			"to":   "user@example.com",
			"body": "Olá, mundo!",
		})
	}

	time.Sleep(5 * time.Second)
}
