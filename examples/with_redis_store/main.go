package main

import (
	"context"
	"log"
	"time"

	"github.com/Thauan/gotsk"
)

func main() {
	store := gotsk.NewRedisStore("localhost:6379", "", 0, "gotsk:queue")
	queue := gotsk.NewWithStore(4, store)

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
