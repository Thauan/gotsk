package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Thauan/gotsk"
	"github.com/Thauan/gotsk/interfaces"
	"github.com/Thauan/gotsk/middlewares"
	"github.com/Thauan/gotsk/store"
)

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	queue := gotsk.NewWithStore(4, store.NewMemoryStore())
	queue.Use(middlewares.LoggingMiddleware(logger))

	wg.Add(1)
	go queue.ServeUI("localhost:8080", ctx)

	queue.Register("send_email", func(ctx context.Context, payload interfaces.Payload) error {
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

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	log.Println("🔴 Encerrando aplicação...")
	cancel()

	queue.Stop()
	wg.Wait()
	log.Println("✅ Finalizado com sucesso")

	// logger := log.New(os.Stderr, "", log.LstdFlags)

	// queue := gotsk.NewWithStore(4, store.NewMemoryStore())
	// go queue.ServeUI("localhost:8080")

	// queue.Use(middlewares.LoggingMiddleware(logger))

	// queue.Register("send_email", func(ctx context.Context, payload interfaces.Payload) error {
	// 	log.Println("Enviando email para:", payload["to"])
	// 	return nil
	// })

	// queue.Start()
	// defer queue.Stop()

	// for range 5 {
	// 	queue.Enqueue("send_email", interfaces.Payload{
	// 		"to":   "user@example.com",
	// 		"body": "Olá, mundo!",
	// 	})
	// }

	// time.Sleep(5 * time.Second)
}
