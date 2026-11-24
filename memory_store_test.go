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

// Ack confirma pelo ID: duas tasks com o mesmo nome e payload não podem ser
// confundidas uma com a outra.
func TestAckMatchesByIDNotPayload(t *testing.T) {
	store := NewMemoryStore()
	require.NoError(t, store.Push(interfaces.Task{ID: "a", Name: "job", Payload: interfaces.Payload{"k": 1}}))
	require.NoError(t, store.Push(interfaces.Task{ID: "b", Name: "job", Payload: interfaces.Payload{"k": 1}}))
	a, err := store.Pop()
	require.NoError(t, err)
	b, err := store.Pop()
	require.NoError(t, err)

	require.NoError(t, store.Ack(b))
	require.Len(t, store.pending, 1)
	require.Equal(t, a.ID, store.pending[0].ID, "só a task confirmada pode sair dos pendentes")
}
