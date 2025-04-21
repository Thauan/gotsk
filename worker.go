package gotsk

import (
	"log"
	"time"
)

func (q *Queue) worker() {
	defer func() {
		q.done <- true
	}()
	defer q.wg.Done()

	for {
		select {
		case <-q.ctx.Done():
			return
		default:
			task, err := q.store.Pop()
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			q.process(task)
		}
	}
}

func (q *Queue) process(task Task) {
	q.mu.RLock()
	handler, ok := q.handlers[task.Name]
	q.mu.RUnlock()

	if !ok {
		log.Printf("no handler for task '%s'", task.Name)
		return
	}

	var err error
	for attempt := 0; attempt <= q.maxRetries; attempt++ {
		err = handler(q.ctx, task.Payload)
		if err == nil {
			return
		}
		log.Printf("task '%s' failed (attempt %d): %v", task.Name, attempt+1, err)
		time.Sleep(simpleBackoff(attempt))
	}

	if err != nil {
		log.Printf("task '%s' failed after %d attempts: %v", task.Name, q.maxRetries+1, err)

		err := q.store.Push(task)
		if err != nil {
			log.Printf("failed to re-enqueue task '%s': %v", task.Name, err)
		}
	}
}
