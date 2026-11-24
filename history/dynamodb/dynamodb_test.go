package dynamodb

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/require"

	"github.com/Thauan/gotsk/interfaces"
)

// Os testes rodam só com DYNAMODB_ENDPOINT definido, por exemplo
// http://localhost:8000 (DynamoDB Local). Sem a variável, são pulados.
func newTestRepo(t *testing.T) (*Repository, *ddb.Client) {
	t.Helper()
	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	if endpoint == "" {
		t.Skip("DYNAMODB_ENDPOINT não definido")
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
	)
	require.NoError(t, err)

	client := ddb.NewFromConfig(cfg, func(o *ddb.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	table := fmt.Sprintf("gotsk_test_%d", time.Now().UnixNano())
	require.NoError(t, EnsureTable(ctx, client, table))
	t.Cleanup(func() {
		client.DeleteTable(context.Background(), &ddb.DeleteTableInput{TableName: aws.String(table)})
	})
	return New(client, table), client
}

func TestDynamoSaveIsUpsertAndListsNewestFirst(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "old", Name: "job", Status: "queued", CreatedAt: now.Add(-time.Hour), Payload: interfaces.Payload{"k": "v"}}))
	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "new", Name: "job", Status: "queued", CreatedAt: now}))
	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "old", Name: "job", Status: "completed", CreatedAt: now.Add(-time.Hour), Payload: interfaces.Payload{"k": "v"}}))

	list, err := repo.List(ctx, interfaces.HistoryFilter{})
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "new", list[0].ID)
	require.Equal(t, "completed", list[1].Status)
	require.Equal(t, "v", list[1].Payload["k"])
}

func TestDynamoFiltersAndCounts(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "a", Name: "send_email", Status: "failed", Retries: 2, CreatedAt: now}))
	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "b", Name: "resize_image", Status: "completed", CreatedAt: now.Add(-48 * time.Hour)}))
	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "c", Name: "send_email", Status: "completed", CreatedAt: now}))

	failed, err := repo.List(ctx, interfaces.HistoryFilter{Status: "failed"})
	require.NoError(t, err)
	require.Len(t, failed, 1)

	byName, err := repo.List(ctx, interfaces.HistoryFilter{Search: "EMAIL"})
	require.NoError(t, err)
	require.Len(t, byName, 2)

	recent, err := repo.List(ctx, interfaces.HistoryFilter{Since: now.Add(-24 * time.Hour)})
	require.NoError(t, err)
	require.Len(t, recent, 2)

	counts, err := repo.Counts(ctx, now.Add(-24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, 1, counts.Failed)
	require.Equal(t, 1, counts.Completed)
	require.Equal(t, 1, counts.Retried)
}

func TestDynamoGetDeleteAndPurge(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "ok1", Name: "job", Status: "completed", CreatedAt: now}))
	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "ok2", Name: "job", Status: "completed", CreatedAt: now}))
	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "bad", Name: "job", Status: "failed", CreatedAt: now}))

	got, err := repo.Get(ctx, "bad")
	require.NoError(t, err)
	require.Equal(t, "failed", got.Status)

	_, err = repo.Get(ctx, "nope")
	require.ErrorIs(t, err, interfaces.ErrTaskNotFound)

	require.NoError(t, repo.Delete(ctx, "ok1"))

	n, err := repo.Purge(ctx, interfaces.HistoryFilter{Status: "completed"})
	require.NoError(t, err)
	require.Equal(t, 1, n)

	left, err := repo.List(ctx, interfaces.HistoryFilter{})
	require.NoError(t, err)
	require.Len(t, left, 1)
	require.Equal(t, "bad", left[0].ID)
}

func TestDynamoExecutionDetailsRoundTrip(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Microsecond)

	in := interfaces.Task{ID: "d1", Name: "job", Status: "failed", Retries: 1, CreatedAt: now,
		StartedAt: now, FinishedAt: now.Add(time.Second), WorkerID: "w1", Error: "boom",
		Attempts: []interfaces.Attempt{
			{Number: 1, StartedAt: now, FinishedAt: now.Add(time.Millisecond), WorkerID: "w1", Error: "boom"},
			{Number: 2, StartedAt: now.Add(time.Second), FinishedAt: now.Add(time.Second), WorkerID: "w1", Error: "boom"},
		}}
	require.NoError(t, repo.Save(ctx, in))

	got, err := repo.Get(ctx, "d1")
	require.NoError(t, err)
	require.Equal(t, "boom", got.Error)
	require.Equal(t, "w1", got.WorkerID)
	require.Len(t, got.Attempts, 2)
	require.Equal(t, 2, got.Attempts[1].Number)
	require.True(t, got.FinishedAt.Equal(now.Add(time.Second)))
}
