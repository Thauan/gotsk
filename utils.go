package gotsk

import (
	"fmt"

	"github.com/google/uuid"
)

// WorkerId returns a unique identifier for a worker.
func WorkerId() string {
	return fmt.Sprintf("worker-%s", uuid.NewString())
}

// TaskId returns a unique identifier for a task.
func TaskId() string {
	return fmt.Sprintf("task-%s", uuid.NewString())
}
