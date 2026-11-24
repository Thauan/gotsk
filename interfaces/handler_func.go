package interfaces

import (
	"context"
)

// HandlerFunc processes a task payload. Returning an error marks the attempt as failed.
type HandlerFunc func(ctx context.Context, payload Payload) error
