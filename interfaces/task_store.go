package interfaces

import "context"

// TaskStore is the storage the queue uses to hold tasks.
// Pop returns the next ready task; Ack confirms that a popped task is done.
type TaskStore interface {
	Push(task Task) error
	Pop() (Task, error)
	Ack(task Task) error
}

// Sized é opcional: stores que sabem quantas tasks estão na fila (aproximado,
// como o SQS) o implementam. A UI usa para mostrar a contagem da dead-letter.
type Sized interface {
	Size(ctx context.Context) (int, error)
}
