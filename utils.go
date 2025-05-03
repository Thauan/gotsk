package gotsk

import (
	"fmt"

	"github.com/google/uuid"
)

func WorkerId() string {
	return fmt.Sprintf("worker-%s", uuid.NewString())
}

func TaskId() string {
	return fmt.Sprintf("task-%s", uuid.NewString())
}
