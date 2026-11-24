package sqs_test

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Thauan/gotsk"
	"github.com/Thauan/gotsk/interfaces"
	sqsstore "github.com/Thauan/gotsk/store/sqs"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/require"
)

// Teste de integração com SQS. Roda só com SQS_ENDPOINT definido, por exemplo
// http://localhost:4566 (LocalStack). Sem a variável, é pulado.
func TestDeadLetterRequeueWithSQS(t *testing.T) {
	endpoint := os.Getenv("SQS_ENDPOINT")
	if endpoint == "" {
		t.Skip("SQS_ENDPOINT não definido")
	}
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
	)
	require.NoError(t, err)
	client := sqs.NewFromConfig(cfg, func(o *sqs.Options) { o.BaseEndpoint = aws.String(endpoint) })

	name := fmt.Sprintf("gotsk-it-%d", time.Now().UnixNano())
	mainURL := createQueue(t, ctx, client, name)
	dlqURL := createQueue(t, ctx, client, name+"-dead")

	queue := gotsk.NewWithStore(1, sqsstore.New(client, mainURL))
	dlq := sqsstore.New(client, dlqURL)
	queue.SetDeadLetter(dlq)

	var failing atomic.Bool
	failing.Store(true)
	var succeeded atomic.Int32
	queue.Register("job", func(ctx context.Context, p interfaces.Payload) error {
		if failing.Load() {
			return fmt.Errorf("falha simulada")
		}
		succeeded.Add(1)
		return nil
	})
	queue.Start()
	defer queue.Stop()

	require.NoError(t, queue.Enqueue("job", interfaces.Payload{"k": "v"}))

	// A task esgota as tentativas e cai na dead-letter.
	require.Eventually(t, func() bool {
		n, err := dlq.Size(ctx)
		return err == nil && n == 1
	}, 90*time.Second, time.Second)

	// Com a falha corrigida, a devolução precisa concluir a task.
	failing.Store(false)
	moved, err := queue.RequeueDead(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, moved)

	require.Eventually(t, func() bool { return succeeded.Load() == 1 }, 90*time.Second, time.Second)
	require.Eventually(t, func() bool {
		n, err := dlq.Size(ctx)
		return err == nil && n == 0
	}, 30*time.Second, time.Second)
}

func createQueue(t *testing.T, ctx context.Context, client *sqs.Client, name string) string {
	t.Helper()
	out, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(name)})
	require.NoError(t, err)
	url := aws.ToString(out.QueueUrl)
	t.Cleanup(func() {
		client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{QueueUrl: aws.String(url)})
	})
	return url
}

// Duas mensagens com o mesmo ID em voo precisam ser confirmadas cada uma com
// o próprio recibo.
func TestAckDuplicateIDsWithSQS(t *testing.T) {
	endpoint := os.Getenv("SQS_ENDPOINT")
	if endpoint == "" {
		t.Skip("SQS_ENDPOINT não definido")
	}
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
	)
	require.NoError(t, err)
	client := sqs.NewFromConfig(cfg, func(o *sqs.Options) { o.BaseEndpoint = aws.String(endpoint) })
	url := createQueue(t, ctx, client, fmt.Sprintf("gotsk-dup-%d", time.Now().UnixNano()))
	store := sqsstore.New(client, url)

	require.NoError(t, store.Push(interfaces.Task{ID: "same", Name: "job"}))
	require.NoError(t, store.Push(interfaces.Task{ID: "same", Name: "job"}))
	first, err := store.Pop()
	require.NoError(t, err)
	second, err := store.Pop()
	require.NoError(t, err)

	require.NoError(t, store.Ack(first))
	require.NoError(t, store.Ack(second))
}
