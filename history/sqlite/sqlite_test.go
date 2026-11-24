package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/stretchr/testify/require"
)

func openTemp(t *testing.T) *Repository {
	t.Helper()
	repo, err := Open("file:" + filepath.Join(t.TempDir(), "gotsk.db"))
	require.NoError(t, err)
	t.Cleanup(func() { repo.Close() })
	return repo
}

func task(id, name, status string, created time.Time) interfaces.Task {
	return interfaces.Task{
		ID:        id,
		Name:      name,
		Status:    status,
		Payload:   interfaces.Payload{"to": "a@b.c"},
		CreatedAt: created,
	}
}

func TestSaveIsUpsertByID(t *testing.T) {
	repo := openTemp(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, repo.Save(ctx, task("t1", "send_email", "queued", now)))
	require.NoError(t, repo.Save(ctx, task("t1", "send_email", "running", now)))
	require.NoError(t, repo.Save(ctx, task("t1", "send_email", "completed", now)))

	list, err := repo.List(ctx, interfaces.HistoryFilter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "completed", list[0].Status)
	require.Equal(t, "a@b.c", list[0].Payload["to"])
}

func TestListFiltersByStatusSearchAndSince(t *testing.T) {
	repo := openTemp(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, repo.Save(ctx, task("t1", "send_email", "completed", now.Add(-48*time.Hour))))
	require.NoError(t, repo.Save(ctx, task("t2", "resize_image", "failed", now.Add(-time.Hour))))
	require.NoError(t, repo.Save(ctx, task("t3", "send_email", "failed", now)))

	failed, err := repo.List(ctx, interfaces.HistoryFilter{Status: "failed"})
	require.NoError(t, err)
	require.Len(t, failed, 2)

	byName, err := repo.List(ctx, interfaces.HistoryFilter{Search: "email"})
	require.NoError(t, err)
	require.Len(t, byName, 2)

	recent, err := repo.List(ctx, interfaces.HistoryFilter{Since: now.Add(-24 * time.Hour)})
	require.NoError(t, err)
	require.Len(t, recent, 2)
	require.Equal(t, "t3", recent[0].ID, "mais recente primeiro")
}

func TestListRespectsLimit(t *testing.T) {
	repo := openTemp(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		require.NoError(t, repo.Save(ctx, task(string(rune('a'+i)), "job", "queued", time.Now())))
	}
	list, err := repo.List(ctx, interfaces.HistoryFilter{Limit: 2})
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestCountsByStatusAndRetries(t *testing.T) {
	repo := openTemp(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, repo.Save(ctx, task("q", "job", "queued", now)))
	require.NoError(t, repo.Save(ctx, task("s", "job", "scheduled", now)))
	require.NoError(t, repo.Save(ctx, task("r", "job", "running", now)))
	require.NoError(t, repo.Save(ctx, task("c", "job", "completed", now)))
	require.NoError(t, repo.Save(ctx, task("f", "job", "failed", now)))

	retried := task("f2", "job", "failed", now)
	retried.Retries = 3
	require.NoError(t, repo.Save(ctx, retried))

	counts, err := repo.Counts(ctx, time.Time{})
	require.NoError(t, err)
	require.Equal(t, 1, counts.Queued)
	require.Equal(t, 1, counts.Scheduled)
	require.Equal(t, 1, counts.Running)
	require.Equal(t, 1, counts.Completed)
	require.Equal(t, 2, counts.Failed)
	require.Equal(t, 1, counts.Retried)
}

func TestCountsRespectsSince(t *testing.T) {
	repo := openTemp(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, repo.Save(ctx, task("old", "job", "completed", now.Add(-72*time.Hour))))
	require.NoError(t, repo.Save(ctx, task("new", "job", "completed", now)))

	counts, err := repo.Counts(ctx, now.Add(-24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, 1, counts.Completed)
}

func TestDataSurvivesReopen(t *testing.T) {
	path := "file:" + filepath.Join(t.TempDir(), "gotsk.db")
	ctx := context.Background()

	repo, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, task("t1", "job", "completed", time.Now())))
	require.NoError(t, repo.Close())

	reopened, err := Open(path)
	require.NoError(t, err)
	defer reopened.Close()
	list, err := reopened.List(ctx, interfaces.HistoryFilter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestGetDeleteAndPurge(t *testing.T) {
	repo := openTemp(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, repo.Save(ctx, task("ok1", "job", "completed", now)))
	require.NoError(t, repo.Save(ctx, task("ok2", "job", "completed", now)))
	require.NoError(t, repo.Save(ctx, task("bad", "job", "failed", now)))

	got, err := repo.Get(ctx, "bad")
	require.NoError(t, err)
	require.Equal(t, "failed", got.Status)

	_, err = repo.Get(ctx, "nope")
	require.ErrorIs(t, err, interfaces.ErrTaskNotFound)

	require.NoError(t, repo.Delete(ctx, "ok1"))
	_, err = repo.Get(ctx, "ok1")
	require.ErrorIs(t, err, interfaces.ErrTaskNotFound)

	n, err := repo.Purge(ctx, interfaces.HistoryFilter{Status: "completed"})
	require.NoError(t, err)
	require.Equal(t, 1, n)

	left, err := repo.List(ctx, interfaces.HistoryFilter{})
	require.NoError(t, err)
	require.Len(t, left, 1)
	require.Equal(t, "bad", left[0].ID)

	n, err = repo.Purge(ctx, interfaces.HistoryFilter{})
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestExecutionDetailsRoundTrip(t *testing.T) {
	repo := openTemp(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Microsecond)

	in := task("t1", "send_email", "failed", now)
	in.Retries = 2
	in.WorkerID = "worker-abc"
	in.StartedAt = now
	in.FinishedAt = now.Add(time.Second)
	in.Error = "smtp timeout"
	in.Attempts = []interfaces.Attempt{
		{Number: 1, StartedAt: now, FinishedAt: now.Add(time.Millisecond * 10), WorkerID: "worker-abc", Error: "smtp timeout"},
		{Number: 2, StartedAt: now.Add(time.Second), FinishedAt: now.Add(time.Second + time.Millisecond), WorkerID: "worker-abc", Error: "smtp timeout"},
	}
	require.NoError(t, repo.Save(ctx, in))

	got, err := repo.Get(ctx, "t1")
	require.NoError(t, err)
	require.Equal(t, "smtp timeout", got.Error)
	require.Equal(t, "worker-abc", got.WorkerID)
	require.True(t, got.StartedAt.Equal(now))
	require.True(t, got.FinishedAt.Equal(now.Add(time.Second)))
	require.Len(t, got.Attempts, 2)
	require.Equal(t, 2, got.Attempts[1].Number)
	require.Equal(t, "smtp timeout", got.Attempts[1].Error)
}

func TestOpenMigratesOldSchema(t *testing.T) {
	path := "file:" + filepath.Join(t.TempDir(), "old.db")
	old, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = old.Exec(`CREATE TABLE tasks (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, status TEXT NOT NULL, payload TEXT NOT NULL,
		retries INTEGER NOT NULL DEFAULT 0, priority INTEGER NOT NULL DEFAULT 0,
		scheduled_at INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL)`)
	require.NoError(t, err)
	_, err = old.Exec(`INSERT INTO tasks VALUES ('legacy', 'job', 'completed', '{}', 0, 0, 0, 1)`)
	require.NoError(t, err)
	require.NoError(t, old.Close())

	repo, err := Open(path)
	require.NoError(t, err)
	defer repo.Close()
	got, err := repo.Get(context.Background(), "legacy")
	require.NoError(t, err)
	require.Equal(t, "completed", got.Status)
	require.Empty(t, got.Attempts)
}
