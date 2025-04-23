package gotsk

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func Run(queue *Queue, addr string) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		queue.ServeUI(addr, ctx)
	}()

	queue.Start()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	log.Println("🔴 Encerrando...")

	cancel()
	queue.Stop()
	wg.Wait()

	log.Println("✅ Finalizado com sucesso")
}
