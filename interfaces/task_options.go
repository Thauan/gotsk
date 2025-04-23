package interfaces

import "time"

type TaskOptions struct {
	Priority    int
	ScheduledAt time.Time
}
