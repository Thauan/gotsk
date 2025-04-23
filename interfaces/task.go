package interfaces

import "time"

type Task struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Payload       Payload   `json:"payload"`
	Retries       int       `json:"retries"`
	ReceiptHandle string    `json:"-"`
	Priority      int       `json:"priority"`
	ScheduledAt   time.Time `json:"scheduled_at"`
}
