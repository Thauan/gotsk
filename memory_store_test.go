package gotsk

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/stretchr/testify/require"
)

// Com um worker, tasks prontas saem pela prioridade (maior primeiro) e
// depois pela ordem de chegada.
func TestPriorityOrdersReadyTasks(t *testing.T) {
	queue := NewWithStore(1, NewMemoryStore())
	var mu sync.Mutex
	var order []int
	queue.Register("job", func(ctx context.Context, p interfaces.Payload) error {
		mu.Lock()
		order = append(order, p["n"].(int))
		mu.Unlock()
		return nil
	})

	// Enfileirado antes de Start, para que todas estejam prontas juntas.
	require.NoError(t, queue.EnqueueAt("job", interfaces.Payload{"n": 1}, interfaces.TaskOptions{Priority: 1}))
	require.NoError(t, queue.EnqueueAt("job", interfaces.Payload{"n": 5}, interfaces.TaskOptions{Priority: 5}))
	require.NoError(t, queue.EnqueueAt("job", interfaces.Payload{"n": 3}, interfaces.TaskOptions{Priority: 3}))
	queue.Start()
	defer queue.Stop()

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(order) == 3
	}, 3*time.Second, 10*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []int{5, 3, 1}, order)
}
