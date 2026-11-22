package sqlite_test

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

// Uma task que roda até o fim deve aparecer no histórico como concluída.
func TestHistoryRecordsCompletedTask(t *testing.T) {
	repo, err := sqlite.Open("file:" + filepath.Join(t.TempDir(), "gotsk.db"))
	require.NoError(t, err)
	defer repo.Close()

	queue := gotsk.NewWithStore(2, gotsk.NewMemoryStore())
	queue.SetHistory(repo)
	queue.Register("send_email", func(ctx context.Context, payload interfaces.Payload) error {
		return nil
	})
	queue.Start()
	defer queue.Stop()

	for i := 0; i < 3; i++ {
		require.NoError(t, queue.Enqueue("send_email", interfaces.Payload{"i": i}))
	}

	require.Eventually(t, func() bool {
		c, err := repo.Counts(context.Background(), time.Time{})
		return err == nil && c.Completed == 3
	}, 5*time.Second, 20*time.Millisecond)

	list, err := repo.List(context.Background(), interfaces.HistoryFilter{Status: "completed"})
	require.NoError(t, err)
	require.Len(t, list, 3)
	for _, task := range list {
		require.Equal(t, "send_email", task.Name)
	}
}
