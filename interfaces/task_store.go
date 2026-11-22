package interfaces

import "context"

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
