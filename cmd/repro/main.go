// Programa de reprodução para investigar a entrega de eventos SSE.
//
// - UI_ADDR (padrão :8080): painel + /events (SSE), servido por gotsk.Run.
// - TASK_DURATION_MS: duração fixa de cada task em ms (padrão: aleatória de 0 a 30).
// - TRIGGER_ADDR (padrão :9090): POST /enqueue?n=N enfileira N tasks em rajada.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Thauan/gotsk"
	"github.com/Thauan/gotsk/interfaces"
	"github.com/Thauan/gotsk/store"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	queue := gotsk.NewWithStore(4, store.NewMemoryStore())

	// TASK_DURATION_MS fixa a duração de cada task; sem ela, cada task dura de 0 a 30 ms.
	fixedMs, _ := strconv.Atoi(env("TASK_DURATION_MS", "0"))

	queue.Register("noop", func(ctx context.Context, payload interfaces.Payload) error {
		if fixedMs > 0 {
			time.Sleep(time.Duration(fixedMs) * time.Millisecond)
		} else {
			time.Sleep(time.Duration(rand.Intn(30)) * time.Millisecond)
		}
		return nil
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/enqueue", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "use POST", http.StatusMethodNotAllowed)
			return
		}
		n, err := strconv.Atoi(r.URL.Query().Get("n"))
		if err != nil || n <= 0 {
			http.Error(w, "parâmetro n inválido", http.StatusBadRequest)
			return
		}
		for i := range n {
			if err := queue.Enqueue("noop", interfaces.Payload{"i": i}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		fmt.Fprintf(w, "enfileiradas %d tasks\n", n)
	})

	triggerAddr := env("TRIGGER_ADDR", ":9090")
	go func() {
		log.Printf("gatilho de rajada em http://%s/enqueue?n=N", triggerAddr)
		log.Fatal(http.ListenAndServe(triggerAddr, mux))
	}()

	gotsk.Run(queue, env("UI_ADDR", ":8080"))
}
