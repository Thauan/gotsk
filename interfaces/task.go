package interfaces

import "time"

type Task struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Payload       Payload   `json:"payload"`
	Status        string    `json:"status"`
	Retries       int       `json:"retries"`
	ReceiptHandle string    `json:"-"`
	Priority      int       `json:"priority"`
	ScheduledAt   time.Time `json:"scheduled_at"`
	CreatedAt     time.Time `json:"created_at"`

	// Dados da execução, preenchidos pelo worker. Ficam vazios para tasks
	// que ainda não rodaram.
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	WorkerID   string    `json:"worker_id,omitempty"`
	Error      string    `json:"error,omitempty"`
	Attempts   []Attempt `json:"attempts,omitempty"`
}

// Attempt é uma tentativa de execução do handler.
type Attempt struct {
	Number     int       `json:"number"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	WorkerID   string    `json:"worker_id"`
	Error      string    `json:"error,omitempty"`
}
