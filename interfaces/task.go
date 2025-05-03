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
}
