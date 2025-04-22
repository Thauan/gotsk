package interfaces

type Queue interface {
	Enqueue(name string, payload Payload) error
	Register(name string, handler HandlerFunc)
	GetWorkers() int
	Start()
	Stop()
}
