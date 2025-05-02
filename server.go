package gotsk

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func Run(queue *Queue) {
	queue.Start()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	log.Println("🔴 Encerrando...")

	queue.Stop()
	log.Println("✅ Finalizado com sucesso")
}
