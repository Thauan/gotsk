package interfaces

type Task struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Payload       Payload `json:"payload"`
	Retries       int     `json:"retries"`
	ReceiptHandle string  `json:"-"`
}
