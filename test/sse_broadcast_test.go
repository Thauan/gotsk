package test

import (
	"context"
	"testing"

	"github.com/Thauan/gotsk"
	"github.com/Thauan/gotsk/interfaces"
	"github.com/stretchr/testify/assert"
)

// Uma rajada de eventos enquanto o cliente SSE ainda não está lendo
// não pode perder eventos: cada cliente deve receber todos eventos
// enfileirados em rajada, mesmo que o streamer esteja ocupado.
func TestBroadcastBurstNotDropped(t *testing.T) {
	const burst = 200

	queue := gotsk.NewWithStore(1, gotsk.NewMemoryStore())
	queue.Register("noop", func(ctx context.Context, payload interfaces.Payload) error {
		return nil
	})

	ch := queue.Subscribe()

	for i := 0; i < burst; i++ {
		assert.NoError(t, queue.Enqueue("noop", interfaces.Payload{"i": i}))
	}

	assert.Equal(t, burst, len(ch), "eventos entregues ao cliente SSE na rajada")
}
