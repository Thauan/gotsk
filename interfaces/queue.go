package interfaces

// Queue is the interface implemented by queues, for code that depends on the behavior
// rather than on the concrete type.
type Queue interface {
	Enqueue(name string, payload Payload) error
	Register(name string, handler HandlerFunc)
	GetWorkers() int
	Start()
	Stop()
}
