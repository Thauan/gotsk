// Package dynamodb implementa interfaces.HistoryRepository sobre DynamoDB.
//
// Tabela: chave de partição "id" (S). Os filtros de status e período são
// feitos com Scan + agregação em memória, então o custo cresce com o tamanho
// da tabela. Para volumes grandes, o próximo passo é um GSI em (status,
// created_at) e consultas com Query.
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/Thauan/gotsk/interfaces"
)

// Item é a forma persistida de uma task.
type item struct {
	ID          string         `dynamodbav:"id"`
	Name        string         `dynamodbav:"name"`
	Status      string         `dynamodbav:"status"`
	Payload     map[string]any `dynamodbav:"payload"`
	Retries     int            `dynamodbav:"retries"`
	Priority    int            `dynamodbav:"priority"`
	ScheduledAt int64          `dynamodbav:"scheduled_at"`
	CreatedAt   int64          `dynamodbav:"created_at"`
	StartedAt   int64          `dynamodbav:"started_at"`
	FinishedAt  int64          `dynamodbav:"finished_at"`
	WorkerID    string         `dynamodbav:"worker_id"`
	Error       string         `dynamodbav:"error"`
	Attempts    []attemptItem  `dynamodbav:"attempts"`
}

type attemptItem struct {
	Number     int    `dynamodbav:"number"`
	StartedAt  int64  `dynamodbav:"started_at"`
	FinishedAt int64  `dynamodbav:"finished_at"`
	WorkerID   string `dynamodbav:"worker_id"`
	Error      string `dynamodbav:"error"`
}

type Repository struct {
	client *ddb.Client
	table  string
}

// New usa um cliente já configurado (região, credenciais e endpoint).
func New(client *ddb.Client, table string) *Repository {
	return &Repository{client: client, table: table}
}

// EnsureTable cria a tabela com chave "id" se ela não existir. Útil em
// desenvolvimento e testes; em produção a tabela costuma ser provisionada
// por IaC.
func EnsureTable(ctx context.Context, client *ddb.Client, table string) error {
	_, err := client.CreateTable(ctx, &ddb.CreateTableInput{
		TableName:   aws.String(table),
		BillingMode: types.BillingModePayPerRequest,
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("id"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS},
		},
	})
	var inUse *types.ResourceInUseException
	if err != nil && !errors.As(err, &inUse) {
		return fmt.Errorf("criando tabela %s: %w", table, err)
	}
	if err != nil {
		return nil // já existe
	}
	waiter := ddb.NewTableExistsWaiter(client)
	return waiter.Wait(ctx, &ddb.DescribeTableInput{TableName: aws.String(table)}, time.Minute)
}

func (r *Repository) Save(ctx context.Context, task interfaces.Task) error {
	av, err := attributevalue.MarshalMap(toItem(task))
	if err != nil {
		return fmt.Errorf("serializando task: %w", err)
	}
	_, err = r.client.PutItem(ctx, &ddb.PutItemInput{
		TableName: aws.String(r.table),
		Item:      av,
	})
	return err
}

func (r *Repository) Get(ctx context.Context, id string) (interfaces.Task, error) {
	out, err := r.client.GetItem(ctx, &ddb.GetItemInput{
		TableName: aws.String(r.table),
		Key:       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: id}},
	})
	if err != nil {
		return interfaces.Task{}, err
	}
	if len(out.Item) == 0 {
		return interfaces.Task{}, interfaces.ErrTaskNotFound
	}
	var it item
	if err := attributevalue.UnmarshalMap(out.Item, &it); err != nil {
		return interfaces.Task{}, err
	}
	return fromItem(it), nil
}

func (r *Repository) List(ctx context.Context, f interfaces.HistoryFilter) ([]interfaces.Task, error) {
	items, err := r.scanAll(ctx, f.Since)
	if err != nil {
		return nil, err
	}

	var tasks []interfaces.Task
	for _, it := range items {
		if matches(it, f) {
			tasks = append(tasks, fromItem(it))
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	limit := f.Limit
	if limit <= 0 {
		limit = interfaces.DefaultHistoryLimit
	}
	if len(tasks) > limit {
		tasks = tasks[:limit]
	}
	return tasks, nil
}

func (r *Repository) Counts(ctx context.Context, since time.Time) (interfaces.HistoryCounts, error) {
	var c interfaces.HistoryCounts
	items, err := r.scanAll(ctx, since)
	if err != nil {
		return c, err
	}
	for _, it := range items {
		switch it.Status {
		case "queued":
			c.Queued++
		case "scheduled":
			c.Scheduled++
		case "running":
			c.Running++
		case "completed":
			c.Completed++
		case "failed":
			c.Failed++
		}
		if it.Retries > 0 {
			c.Retried++
		}
	}
	return c, nil
}

func matches(it item, f interfaces.HistoryFilter) bool {
	if f.Status != "" && it.Status != f.Status {
		return false
	}
	if f.Search != "" {
		search := strings.ToLower(f.Search)
		if !strings.Contains(strings.ToLower(it.ID), search) &&
			!strings.Contains(strings.ToLower(it.Name), search) {
			return false
		}
	}
	return true
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.client.DeleteItem(ctx, &ddb.DeleteItemInput{
		TableName: aws.String(r.table),
		Key:       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: id}},
	})
	return err
}

// Purge apaga em lotes de 25 (limite do BatchWriteItem). Cada lote é
// reenviado enquanto a AWS devolver itens não processados.
func (r *Repository) Purge(ctx context.Context, f interfaces.HistoryFilter) (int, error) {
	items, err := r.scanAll(ctx, f.Since)
	if err != nil {
		return 0, err
	}

	var ids []string
	for _, it := range items {
		if matches(it, f) {
			ids = append(ids, it.ID)
		}
	}

	const batchSize = 25
	for start := 0; start < len(ids); start += batchSize {
		end := min(start+batchSize, len(ids))
		var reqs []types.WriteRequest
		for _, id := range ids[start:end] {
			reqs = append(reqs, types.WriteRequest{DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: id}},
			}})
		}
		pending := map[string][]types.WriteRequest{r.table: reqs}
		for attempt := 0; len(pending) > 0; attempt++ {
			if attempt > 5 {
				return 0, errors.New("dynamodb: itens não processados após 5 tentativas")
			}
			out, err := r.client.BatchWriteItem(ctx, &ddb.BatchWriteItemInput{RequestItems: pending})
			if err != nil {
				return 0, fmt.Errorf("apagando tasks: %w", err)
			}
			pending = out.UnprocessedItems
		}
	}
	return len(ids), nil
}

// scanAll lê todas as páginas, filtrando por período no servidor.
func (r *Repository) scanAll(ctx context.Context, since time.Time) ([]item, error) {
	input := &ddb.ScanInput{TableName: aws.String(r.table)}
	if !since.IsZero() {
		input.FilterExpression = aws.String("created_at >= :since")
		input.ExpressionAttributeValues = map[string]types.AttributeValue{
			":since": &types.AttributeValueMemberN{Value: fmt.Sprint(since.UnixNano())},
		}
	}

	var items []item
	paginator := ddb.NewScanPaginator(r.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("lendo tasks: %w", err)
		}
		var batch []item
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &batch); err != nil {
			return nil, fmt.Errorf("decodificando tasks: %w", err)
		}
		items = append(items, batch...)
	}
	return items, nil
}

func toItem(t interfaces.Task) item {
	attempts := make([]attemptItem, 0, len(t.Attempts))
	for _, a := range t.Attempts {
		attempts = append(attempts, attemptItem{
			Number:     a.Number,
			StartedAt:  toUnixNano(a.StartedAt),
			FinishedAt: toUnixNano(a.FinishedAt),
			WorkerID:   a.WorkerID,
			Error:      a.Error,
		})
	}
	return item{
		ID:          t.ID,
		Name:        t.Name,
		Status:      t.Status,
		Payload:     t.Payload,
		Retries:     t.Retries,
		Priority:    t.Priority,
		ScheduledAt: toUnixNano(t.ScheduledAt),
		CreatedAt:   toUnixNano(t.CreatedAt),
		StartedAt:   toUnixNano(t.StartedAt),
		FinishedAt:  toUnixNano(t.FinishedAt),
		WorkerID:    t.WorkerID,
		Error:       t.Error,
		Attempts:    attempts,
	}
}

func fromItem(it item) interfaces.Task {
	var attempts []interfaces.Attempt
	for _, a := range it.Attempts {
		attempts = append(attempts, interfaces.Attempt{
			Number:     a.Number,
			StartedAt:  fromUnixNano(a.StartedAt),
			FinishedAt: fromUnixNano(a.FinishedAt),
			WorkerID:   a.WorkerID,
			Error:      a.Error,
		})
	}
	return interfaces.Task{
		ID:          it.ID,
		Name:        it.Name,
		Status:      it.Status,
		Payload:     it.Payload,
		Retries:     it.Retries,
		Priority:    it.Priority,
		ScheduledAt: fromUnixNano(it.ScheduledAt),
		CreatedAt:   fromUnixNano(it.CreatedAt),
		StartedAt:   fromUnixNano(it.StartedAt),
		FinishedAt:  fromUnixNano(it.FinishedAt),
		WorkerID:    it.WorkerID,
		Error:       it.Error,
		Attempts:    attempts,
	}
}

func toUnixNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

func fromUnixNano(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// Name identifica o backend na interface de configurações.
func (r *Repository) Name() string { return "DynamoDB (" + r.table + ")" }
