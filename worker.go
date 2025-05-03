package gotsk

import (
	"log"
	"time"

	"github.com/Thauan/gotsk/interfaces"
)

func (q *Queue) worker() {
	defer q.wg.Done()

	workerID := WorkerId()
	log.Printf("👷 Worker %s iniciado", workerID)

	for {
		select {
		case <-q.ctx.Done():
			log.Printf("🛑 Worker %s encerrado", workerID)
			return
		default:
			task, err := q.store.Pop()
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if !task.ScheduledAt.IsZero() && task.ScheduledAt.After(time.Now()) {
				_ = q.store.Push(task)

				sleepFor := min(time.Until(task.ScheduledAt), time.Second)
				time.Sleep(sleepFor)
				continue
			}

			q.process(task, workerID)
		}
	}
}

func (q *Queue) process(task interfaces.Task, workerID string) {
	q.mu.RLock()
	handler, ok := q.handlers[task.Name]
	q.mu.RUnlock()

	if !ok {
		log.Printf("⚠️ Worker %s: handler não registrado para task '%s'", workerID, task.Name)
		return
	}

	for i := len(q.middlewares) - 1; i >= 0; i-- {
		handler = HandlerFunc(q.middlewares[i](interfaces.HandlerFunc(handler)))
	}

	log.Printf("🚀 Worker %s: processando task %s (%s)", workerID, task.ID, task.Name)

	task.Status = "running"
	q.broadcast(task)

	var err error
	for attempt := 0; attempt <= q.maxRetries; attempt++ {
		err = handler(q.ctx, task.Payload)
		if err == nil {
			task.Status = "completed"
			q.AddToHistory(task)
			q.store.Ack(task)
			q.broadcast(task)
			log.Printf("✅ Worker %s: task %s concluída", workerID, task.ID)
			return
		}
		log.Printf("❌ Worker %s: task %s falhou (tentativa %d): %v", workerID, task.ID, attempt+1, err)
		time.Sleep(simpleBackoff(attempt))
	}

	task.Status = "failed"
	q.broadcast(task)

	log.Printf("💥 Worker %s: task %s falhou após %d tentativas", workerID, task.ID, q.maxRetries+1)
}
