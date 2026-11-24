package interfaces

import "time"

// TaskOptions controls how a task is scheduled.
type TaskOptions struct {
	Priority    int
	ScheduledAt time.Time
}
