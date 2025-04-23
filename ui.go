package gotsk

import (
	"log"

	"github.com/Thauan/gotsk/handlers"
)

func StartUI(addr string) {
	go func() {
		log.Printf("🖥️ Servindo painel de tarefas em http://%s", addr)
		if err := handlers.StartServer(addr); err != nil {
			log.Fatalf("Erro ao iniciar UI: %v", err)
		}
	}()
}
