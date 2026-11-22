package gotsk

import (
	"context"
	"time"
)

func simpleBackoff(attempt int) time.Duration {
	return time.Second * time.Duration(attempt+1)
}

// sleepCtx espera d, ou menos se ctx for cancelado. Devolve false se foi
// cancelado, para que quem espera possa encerrar sem atrasar o shutdown.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
