package gotsk

import (
	"log"
	"time"

	"github.com/Thauan/gotsk/interfaces"
)

func (q *Queue) worker() {
	defer q.wg.Done()
	log.Println("👷 Worker iniciado")

	for {
		select {
		case <-q.ctx.Done():
			log.Println("🛑 Worker encerrado")
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

			q.process(task)
		}
	}
}

func (q *Queue) process(task interfaces.Task) {
	q.mu.RLock()
	handler, ok := q.handlers[task.Name]
	q.mu.RUnlock()

	if !ok {
		log.Printf("no handler for task '%s'", task.Name)
		return
	}

	for i := len(q.middlewares) - 1; i >= 0; i-- {
		handler = HandlerFunc(q.middlewares[i](interfaces.HandlerFunc(handler)))
	}

	var err error
	for attempt := 0; attempt <= q.maxRetries; attempt++ {
		err = handler(q.ctx, task.Payload)
		if err == nil {
			q.store.Ack(task)
			return
		}
		log.Printf("task '%s' failed (attempt %d): %v", task.Name, attempt+1, err)
		time.Sleep(simpleBackoff(attempt))
	}
	log.Printf("task '%s' failed after %d attempts: %v", task.Name, q.maxRetries+1, err)
}
