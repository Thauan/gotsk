package gotsk

import (
	"context"
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
