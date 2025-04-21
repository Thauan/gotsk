package interfaces

type TaskStore interface {
	Push(task Task) error
	Pop() (Task, error)
	Ack(task Task) error
}
