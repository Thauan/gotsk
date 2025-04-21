package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Thauan/gotsk"
	"github.com/Thauan/gotsk/interfaces"
	"github.com/stretchr/testify/assert"
)

func TestRegister(t *testing.T) {
	queue := gotsk.NewWithStore(2, gotsk.NewMemoryStore())

	queue.Register("test_task", func(ctx context.Context, payload interfaces.Payload) error {
		assert.Equal(t, "test", payload["key"])
		return nil
	})

	err := queue.Enqueue("test_task", interfaces.Payload{"key": "test"})
	assert.NoError(t, err)
}

func TestEnqueue(t *testing.T) {
	store := gotsk.NewMemoryStore()
	queue := gotsk.NewWithStore(2, store)

	queue.Register("test_task", func(ctx context.Context, payload interfaces.Payload) error {
		assert.Equal(t, "test", payload["key"])
		return nil
	})

	err := queue.Enqueue("test_task", interfaces.Payload{"key": "test"})
	assert.NoError(t, err)

	queue.Start()
	time.Sleep(500 * time.Millisecond)
	queue.Stop()

	assert.Equal(t, 0, store.LenQueue())
	assert.Equal(t, 0, store.LenPending())

}

func TestStartStop(t *testing.T) {
	store := gotsk.NewMemoryStore()
	queue := gotsk.NewWithStore(2, store)

	queue.Register("test_task", func(ctx context.Context, payload interfaces.Payload) error {
		assert.Equal(t, "test", payload["key"])
		return nil
	})

	err := queue.Enqueue("test_task", interfaces.Payload{"key": "test"})
	assert.NoError(t, err)

	queue.Start()
	time.Sleep(500 * time.Millisecond)
	queue.Stop()

	assert.Equal(t, 0, store.LenQueue())
	assert.Equal(t, 0, store.LenPending())
	assert.Equal(t, 2, queue.GetWorkers())
}

func TestFailedTask(t *testing.T) {
	store := gotsk.NewMemoryStore()
	queue := gotsk.NewWithStore(2, store)

	queue.Register("fail_task", func(ctx context.Context, payload interfaces.Payload) error {
		return fmt.Errorf("task intentionally failed")
	})

	err := queue.Enqueue("fail_task", interfaces.Payload{"key": "test"})
	assert.NoError(t, err)

	queue.Start()
	time.Sleep(5 * time.Second)
	queue.Stop()

	assert.Equal(t, 0, store.LenQueue())
	assert.Equal(t, 1, store.LenPending())
}
