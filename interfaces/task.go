package interfaces

type Payload map[string]interface{}

type Task struct {
	Name    string
	Payload Payload
	Retries int
}
