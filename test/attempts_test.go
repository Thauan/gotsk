package test

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Thauan/gotsk"
	"github.com/Thauan/gotsk/history/sqlite"
	"github.com/Thauan/gotsk/interfaces"
	"github.com/stretchr/testify/require"
)

// Uma task que falha uma vez e depois passa deve guardar as duas tentativas,
// com o erro da primeira, o worker e os horários de execução.
func TestAttemptsAreRecorded(t *testing.T) {
	repo, err := sqlite.Open("file:" + filepath.Join(t.TempDir(), "gotsk.db"))
	require.NoError(t, err)
	defer repo.Close()

	var calls atomic.Int32
	queue := gotsk.NewWithStore(1, gotsk.NewMemoryStore())
	queue.SetHistory(repo)
	queue.Register("flaky", func(ctx context.Context, p interfaces.Payload) error {
		if calls.Add(1) == 1 {
			return errors.New("timeout no gateway")
		}
		return nil
	})
	queue.Start()
	defer queue.Stop()

	require.NoError(t, queue.Enqueue("flaky", interfaces.Payload{"order": 42}))

	var id string
	require.Eventually(t, func() bool {
		list, err := repo.List(context.Background(), interfaces.HistoryFilter{Status: "completed"})
		if err != nil || len(list) != 1 {
			return false
		}
		id = list[0].ID
		return true
	}, 5*time.Second, 20*time.Millisecond)

	got, err := repo.Get(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "completed", got.Status)
	require.Empty(t, got.Error, "erro final vazio quando a task termina com sucesso")
	require.NotEmpty(t, got.WorkerID)
	require.False(t, got.StartedAt.IsZero())
	require.False(t, got.FinishedAt.IsZero())
	require.Equal(t, 42, int(got.Payload["order"].(float64)))

	require.Len(t, got.Attempts, 2)
	require.Equal(t, 1, got.Attempts[0].Number)
	require.Equal(t, "timeout no gateway", got.Attempts[0].Error)
	require.Equal(t, 2, got.Attempts[1].Number)
	require.Empty(t, got.Attempts[1].Error)
	require.Equal(t, got.WorkerID, got.Attempts[0].WorkerID)
}
