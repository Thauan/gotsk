package interfaces

import (
	"context"
)

type HandlerFunc func(ctx context.Context, payload Payload) error
