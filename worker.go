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
			if q.paused.Load() {
				sleepCtx(q.ctx, 200*time.Millisecond)
				continue
			}
			task, err := q.store.Pop()
			if err != nil {
				sleepCtx(q.ctx, 500*time.Millisecond)
				continue
			}

			if !task.ScheduledAt.IsZero() && task.ScheduledAt.After(time.Now()) {
				_ = q.store.Push(task)

				sleepFor := min(time.Until(task.ScheduledAt), time.Second)
				sleepCtx(q.ctx, sleepFor)
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
	task.WorkerID = workerID
	task.StartedAt = time.Now()
	q.publish(task)

	for attempt := 0; attempt <= q.maxRetries; attempt++ {
		started := time.Now()
		err := handler(q.ctx, task.Payload)
		record := interfaces.Attempt{
			Number:     attempt + 1,
			StartedAt:  started,
			FinishedAt: time.Now(),
			WorkerID:   workerID,
		}

		if err == nil {
			task.Attempts = append(task.Attempts, record)
			task.Status = "completed"
			task.FinishedAt = record.FinishedAt
			task.Error = ""
			q.store.Ack(task)
			q.publish(task)
			log.Printf("✅ Worker %s: task %s concluída", workerID, task.ID)
			return
		}

		record.Error = err.Error()
		task.Attempts = append(task.Attempts, record)
		task.Error = record.Error
		log.Printf("❌ Worker %s: task %s falhou (tentativa %d): %v", workerID, task.ID, attempt+1, err)
		if attempt < q.maxRetries {
			task.Retries = attempt + 1
			q.publish(task)
		}
		if attempt == q.maxRetries {
			break // última tentativa: não há o que esperar
		}
		if !sleepCtx(q.ctx, simpleBackoff(attempt)) {
			// Encerramento durante a retentativa: a task não é confirmada,
			// então um store durável (SQS, Redis) a entrega de novo depois.
			task.Status = "failed"
			task.FinishedAt = time.Now()
			task.Error = "fila encerrada durante retentativa"
			q.publish(task)
			log.Printf("🛑 Worker %s: task %s interrompida pelo encerramento", workerID, task.ID)
			return
		}
	}

	task.Status = "failed"
	task.FinishedAt = time.Now()
	q.publish(task)
	q.deadLetter(task, workerID)

	log.Printf("💥 Worker %s: task %s falhou após %d tentativas", workerID, task.ID, q.maxRetries+1)
}

// deadLetter tira a task da fila principal. Com dead-letter configurada, a
// task é gravada lá antes; se a gravação falhar, a task não é confirmada e
// o store a entrega de novo. Sem dead-letter, a task é descartada da fila
// (o registro continua no histórico), em vez de ser reentregue para sempre.
func (q *Queue) deadLetter(task interfaces.Task, workerID string) {
	if q.dlq != nil {
		if err := q.dlq.Push(task); err != nil {
			log.Printf("⚠️ Worker %s: task %s não foi para a dead-letter (%v); será entregue de novo", workerID, task.ID, err)
			return
		}
	}
	if err := q.store.Ack(task); err != nil {
		log.Printf("erro ao confirmar task %s após falha definitiva: %v", task.ID, err)
	}
}
