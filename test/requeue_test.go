package test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Thauan/gotsk"
	"github.com/Thauan/gotsk/history/sqlite"
	"github.com/Thauan/gotsk/interfaces"
	"github.com/stretchr/testify/require"
)

func TestPauseHoldsNewTasksUntilResume(t *testing.T) {
	repo, err := sqlite.Open("file:" + filepath.Join(t.TempDir(), "gotsk.db"))
	require.NoError(t, err)
	defer repo.Close()

	queue := gotsk.NewWithStore(2, gotsk.NewMemoryStore())
	queue.SetHistory(repo)
	queue.Register("job", func(ctx context.Context, p interfaces.Payload) error { return nil })
	queue.Start()
	defer queue.Stop()

	queue.Pause()
	require.NoError(t, queue.Enqueue("job", interfaces.Payload{}))

	time.Sleep(400 * time.Millisecond)
	c, err := repo.Counts(context.Background(), time.Time{})
	require.NoError(t, err)
	require.Equal(t, 0, c.Completed, "pausada: nada deve ser processado")
	require.Equal(t, 1, c.Queued)

	queue.Resume()
	require.Eventually(t, func() bool {
		c, err := repo.Counts(context.Background(), time.Time{})
		return err == nil && c.Completed == 1
	}, 3*time.Second, 20*time.Millisecond)
}

func TestRequeueFailedReprocessesAndSkipsUnknownHandlers(t *testing.T) {
	repo, err := sqlite.Open("file:" + filepath.Join(t.TempDir(), "gotsk.db"))
	require.NoError(t, err)
	defer repo.Close()
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "f1", Name: "job", Status: "failed", Payload: interfaces.Payload{"k": "v"}, CreatedAt: time.Now()}))
	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "f2", Name: "sem_handler", Status: "failed", CreatedAt: time.Now()}))

	queue := gotsk.NewWithStore(2, gotsk.NewMemoryStore())
	queue.SetHistory(repo)
	queue.Register("job", func(ctx context.Context, p interfaces.Payload) error { return nil })
	queue.Start()
	defer queue.Stop()

	requeued, skipped, err := queue.RequeueFailed(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, requeued)
	require.Equal(t, 1, skipped)

	_, err = repo.Get(ctx, "f1")
	require.ErrorIs(t, err, interfaces.ErrTaskNotFound, "falha reenfileirada sai do histórico de falhas")

	_, err = repo.Get(ctx, "f2")
	require.NoError(t, err, "sem handler, a falha continua registrada")

	require.Eventually(t, func() bool {
		c, err := repo.Counts(ctx, time.Time{})
		return err == nil && c.Completed == 1
	}, 3*time.Second, 20*time.Millisecond)
}
