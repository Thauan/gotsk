package gotsk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/stretchr/testify/require"
)

// Parar a fila não pode esperar o backoff de uma task que está em retentativa.
func TestStopDoesNotWaitForRetryBackoff(t *testing.T) {
	queue := NewWithStore(1, NewMemoryStore())
	attempted := make(chan struct{}, 1)
	queue.Register("flaky", func(ctx context.Context, p interfaces.Payload) error {
		select {
		case attempted <- struct{}{}:
		default:
		}
		return errors.New("falha")
	})
	queue.Start()

	require.NoError(t, queue.Enqueue("flaky", interfaces.Payload{}))
	waitFor(t, attempted)
	time.Sleep(100 * time.Millisecond) // o worker está dormindo no backoff

	start := time.Now()
	queue.Stop()
	require.Less(t, time.Since(start), 400*time.Millisecond, "Stop esperou o backoff de retentativa")
}
