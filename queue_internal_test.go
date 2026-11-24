package gotsk

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/stretchr/testify/require"
)

// Um middleware deve rodar uma vez por task, não uma vez a cada registro
// mais uma vez a cada execução.
func TestMiddlewareRunsOncePerTask(t *testing.T) {
	queue := NewWithStore(1, NewMemoryStore())
	var calls int
	var mu sync.Mutex
	queue.Use(func(next interfaces.HandlerFunc) interfaces.HandlerFunc {
		return func(ctx context.Context, p interfaces.Payload) error {
			mu.Lock()
			calls++
			mu.Unlock()
			return next(ctx, p)
		}
	})
	done := make(chan struct{})
	queue.Register("job", func(ctx context.Context, p interfaces.Payload) error {
		close(done)
		return nil
	})
	queue.Start()
	defer queue.Stop()

	require.NoError(t, queue.Enqueue("job", interfaces.Payload{}))
	waitFor(t, done)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, calls)
}

// Middleware adicionado depois de Register também deve valer para o handler.
func TestMiddlewareAddedAfterRegisterApplies(t *testing.T) {
	queue := NewWithStore(1, NewMemoryStore())
	queue.Register("job", func(ctx context.Context, p interfaces.Payload) error { return nil })

	applied := make(chan struct{}, 1)
	queue.Use(func(next interfaces.HandlerFunc) interfaces.HandlerFunc {
		return func(ctx context.Context, p interfaces.Payload) error {
			select {
			case applied <- struct{}{}:
			default:
			}
			return next(ctx, p)
		}
	})
	queue.Start()
	defer queue.Stop()

	require.NoError(t, queue.Enqueue("job", interfaces.Payload{}))
	waitFor(t, applied)
}

// Um Push lento (ex.: banco de histórico) não pode impedir Register.
type slowStore struct {
	*MemoryStore
	delay time.Duration
}

func (s *slowStore) Push(task interfaces.Task) error {
	time.Sleep(s.delay)
	return s.MemoryStore.Push(task)
}

func TestSlowEnqueueDoesNotBlockRegister(t *testing.T) {
	store := &slowStore{MemoryStore: NewMemoryStore(), delay: 500 * time.Millisecond}
	queue := NewWithStore(1, store)
	queue.Register("job", func(ctx context.Context, p interfaces.Payload) error { return nil })

	go queue.Enqueue("job", interfaces.Payload{})
	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	queue.Register("other", func(ctx context.Context, p interfaces.Payload) error { return nil })
	require.Less(t, time.Since(start), 200*time.Millisecond, "Register ficou bloqueado pelo Enqueue")
}

func waitFor[T any](t *testing.T, ch <-chan T) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("tempo esgotado esperando o evento")
	}
}

// RequeueDead devolve as tasks da dead-letter para a fila principal.
func TestRequeueDeadMovesTasksBack(t *testing.T) {
	store := NewMemoryStore()
	dlq := NewMemoryStore()
	queue := NewWithStore(1, store)
	queue.SetDeadLetter(dlq)
	queue.Register("job", func(ctx context.Context, p interfaces.Payload) error { return nil })

	for i := 0; i < 3; i++ {
		require.NoError(t, dlq.Push(interfaces.Task{ID: fmt.Sprintf("dead-%d", i), Name: "job", Status: "failed", Payload: interfaces.Payload{"i": i}}))
	}

	moved, err := queue.RequeueDead(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, moved)
	require.Equal(t, 3, store.LenQueue())
	require.Equal(t, 0, dlq.LenQueue())
}

func TestRequeueDeadWithoutDeadLetter(t *testing.T) {
	queue := NewWithStore(1, NewMemoryStore())
	_, err := queue.RequeueDead(context.Background())
	require.ErrorIs(t, err, ErrNoDeadLetter)
}

// DeadLetterSize conta a dead-letter quando o store sabe contar.
func TestDeadLetterSize(t *testing.T) {
	dlq := NewMemoryStore()
	queue := NewWithStore(1, NewMemoryStore())

	require.False(t, queue.DeadLetterConfigured())

	queue.SetDeadLetter(dlq)
	require.True(t, queue.DeadLetterConfigured())

	require.NoError(t, dlq.Push(interfaces.Task{ID: "a", Name: "job", Status: "failed"}))
	require.NoError(t, dlq.Push(interfaces.Task{ID: "b", Name: "job", Status: "failed"}))

	n, ok, err := queue.DeadLetterSize(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 2, n)
}

// Uma dead-letter que sempre devolve a mesma task (ela volta a falhar e
// retorna durante a chamada) não pode fazer RequeueDead rodar em loop.
type repeatingDead struct {
	*MemoryStore
	task interfaces.Task
}

func (d *repeatingDead) Pop() (interfaces.Task, error)     { return d.task, nil }
func (d *repeatingDead) Ack(interfaces.Task) error         { return nil }
func (d *repeatingDead) Size(context.Context) (int, error) { return 1, nil }

func TestRequeueDeadDoesNotLoop(t *testing.T) {
	store := NewMemoryStore()
	dlq := &repeatingDead{MemoryStore: NewMemoryStore(), task: interfaces.Task{ID: "same", Name: "job"}}
	queue := NewWithStore(1, store)
	queue.SetDeadLetter(dlq)
	queue.Register("job", func(ctx context.Context, p interfaces.Payload) error { return nil })

	moved, err := queue.RequeueDead(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, moved, "cada task é movida uma vez por chamada")
	require.Equal(t, 1, store.LenQueue())
}

// Contagem atrasada: o store diz que está vazio, mas tem tasks (é o que
// acontece com ApproximateNumberOfMessages do SQS logo após um envio).
// RequeueDead não pode depender dessa contagem para decidir o que devolver.
type staleCountDead struct{ *MemoryStore }

func (d *staleCountDead) Size(context.Context) (int, error) { return 0, nil }

func TestRequeueDeadIgnoresStaleCount(t *testing.T) {
	store := NewMemoryStore()
	dlq := &staleCountDead{NewMemoryStore()}
	queue := NewWithStore(1, store)
	queue.SetDeadLetter(dlq)
	queue.Register("job", func(ctx context.Context, p interfaces.Payload) error { return nil })
	require.NoError(t, dlq.Push(interfaces.Task{ID: "a", Name: "job"}))
	require.NoError(t, dlq.Push(interfaces.Task{ID: "b", Name: "job"}))

	moved, err := queue.RequeueDead(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, moved)
	require.Equal(t, 2, store.LenQueue())
}
