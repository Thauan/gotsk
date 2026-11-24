package gotsk

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// Run inicia os workers e o painel em addr, e encerra de forma ordenada
// com SIGINT ou SIGTERM.
func Run(queue *Queue, addr string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue.ServeUI(addr, ctx)
	queue.Start()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	log.Println("🔴 Shutting down...")

	cancel()
	queue.Stop()
	log.Println("✅ Stopped cleanly")
}
