package gotsk

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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

// recordingStore registra as confirmações (Ack) feitas no store.
type recordingStore struct {
	*MemoryStore
	mu   sync.Mutex
	acks int
}

func (s *recordingStore) Ack(task interfaces.Task) error {
	s.mu.Lock()
	s.acks++
	s.mu.Unlock()
	return s.MemoryStore.Ack(task)
}

func (s *recordingStore) ackCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.acks
}

// Uma task que esgota as tentativas vai para a dead-letter e só então é
// confirmada na fila principal, para não ser reentregue para sempre.
func TestFailedTaskGoesToDeadLetter(t *testing.T) {
	store := &recordingStore{MemoryStore: NewMemoryStore()}
	dlq := NewMemoryStore()
	queue := NewWithStore(1, store)
	queue.SetDeadLetter(dlq)
	queue.maxRetries = 0
	queue.Register("flaky", func(ctx context.Context, p interfaces.Payload) error {
		return errors.New("falha")
	})
	queue.Start()
	defer queue.Stop()

	require.NoError(t, queue.Enqueue("flaky", interfaces.Payload{"k": "v"}))
	require.Eventually(t, func() bool {
		return dlq.LenQueue() == 1 && store.ackCount() == 1
	}, 3*time.Second, 10*time.Millisecond)
}

// Se a dead-letter não aceita a task, ela não pode ser confirmada na fila
// principal: continua lá para ser entregue de novo.
type failingPushStore struct{ *MemoryStore }

func (s *failingPushStore) Push(task interfaces.Task) error { return errors.New("dlq indisponível") }

func TestDeadLetterFailureKeepsTaskInMainQueue(t *testing.T) {
	store := &recordingStore{MemoryStore: NewMemoryStore()}
	queue := NewWithStore(1, store)
	queue.SetDeadLetter(&failingPushStore{NewMemoryStore()})
	queue.maxRetries = 0
	queue.Register("flaky", func(ctx context.Context, p interfaces.Payload) error {
		return errors.New("falha")
	})
	queue.Start()
	defer queue.Stop()

	require.NoError(t, queue.Enqueue("flaky", interfaces.Payload{}))
	time.Sleep(300 * time.Millisecond)
	require.Equal(t, 0, store.ackCount(), "task não pode ser confirmada sem chegar à dead-letter")
}

// Sem dead-letter, a task é confirmada (e não reentregue), e o registro fica no histórico.
func TestFailedTaskWithoutDeadLetterIsAcked(t *testing.T) {
	store := &recordingStore{MemoryStore: NewMemoryStore()}
	queue := NewWithStore(1, store)
	queue.maxRetries = 0
	queue.Register("flaky", func(ctx context.Context, p interfaces.Payload) error {
		return errors.New("falha")
	})
	queue.Start()
	defer queue.Stop()

	require.NoError(t, queue.Enqueue("flaky", interfaces.Payload{}))
	require.Eventually(t, func() bool { return store.ackCount() == 1 }, 3*time.Second, 10*time.Millisecond)
}

// Uma task que foi para a dead-letter, quando reprocessada com sucesso,
// deve ser concluída e sair da dead-letter.
func TestDeadLetterRequeueCompletesTask(t *testing.T) {
	store := NewMemoryStore()
	dlq := NewMemoryStore()
	queue := NewWithStore(1, store)
	queue.SetDeadLetter(dlq)

	var failing atomic.Bool
	failing.Store(true)
	var succeeded atomic.Int32
	queue.Register("job", func(ctx context.Context, p interfaces.Payload) error {
		if failing.Load() {
			return errors.New("falha")
		}
		succeeded.Add(1)
		return nil
	})
	queue.Start()
	defer queue.Stop()

	require.NoError(t, queue.Enqueue("job", interfaces.Payload{}))
	require.Eventually(t, func() bool { return dlq.LenQueue() == 1 }, 10*time.Second, 10*time.Millisecond)

	failing.Store(false)
	moved, err := queue.RequeueDead(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, moved)

	require.Eventually(t, func() bool { return succeeded.Load() == 1 }, 5*time.Second, 10*time.Millisecond)
	require.Equal(t, 0, dlq.LenQueue(), "task concluída não pode continuar na dead-letter")
}
