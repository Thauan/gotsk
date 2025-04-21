package gotsk

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRegister(t *testing.T) {
	queue := NewWithStore(2, NewMemoryStore())

	queue.Register("test_task", func(ctx context.Context, payload Payload) error {
		assert.Equal(t, "test", payload["key"])
		return nil
	})

	err := queue.Enqueue("test_task", Payload{"key": "test"})
	assert.NoError(t, err)
}

func TestEnqueue(t *testing.T) {
	store := NewMemoryStore()
	queue := NewWithStore(2, store)

	queue.Register("test_task", func(ctx context.Context, payload Payload) error {
		assert.Equal(t, "test", payload["key"])
		return nil
	})

	err := queue.Enqueue("test_task", Payload{"key": "test"})
	assert.NoError(t, err)

	queue.Start()

	time.Sleep(500 * time.Millisecond)

	assert.Len(t, store.tasks, 0)

	queue.Stop()
}

func TestStartStop(t *testing.T) {
	store := NewMemoryStore()
	queue := NewWithStore(2, store)

	queue.Register("test_task", func(ctx context.Context, payload Payload) error {
		assert.Equal(t, "test", payload["key"])
		return nil
	})

	err := queue.Enqueue("test_task", Payload{"key": "test"})
	assert.NoError(t, err)

	queue.Start()

	assert.Equal(t, 2, queue.workers)

	time.Sleep(500 * time.Millisecond)

	assert.Len(t, store.tasks, 0)

	queue.Stop()

	select {
	case <-queue.done:
	default:
		t.Fatal("Canal 'done' não foi fechado")
	}
}

func TestFailedTask(t *testing.T) {
	store := NewMemoryStore()
	queue := NewWithStore(2, store)

	queue.Register("fail_task", func(ctx context.Context, payload Payload) error {
		return fmt.Errorf("intencionalmente falhou")
	})

	err := queue.Enqueue("fail_task", Payload{"key": "test"})
	assert.NoError(t, err)

	queue.Start()

	time.Sleep(5 * time.Second)

	assert.Len(t, store.tasks, 1)
	queue.Stop()
}
