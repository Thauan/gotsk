package interfaces

import (
	"context"
	"errors"
	"time"
)

// ErrTaskNotFound é retornado por Get quando a task não existe no histórico.
var ErrTaskNotFound = errors.New("task não encontrada")

// HistoryRepository persiste o ciclo de vida das tasks para consulta na UI.
// Ele é separado do TaskStore (que despacha trabalho): o TaskStore decide o
// que roda; o HistoryRepository guarda o que aconteceu.
//
// Save é um upsert pelo ID: cada mudança de status sobrescreve o registro.
type HistoryRepository interface {
	Save(ctx context.Context, task Task) error
	Get(ctx context.Context, id string) (Task, error)
	List(ctx context.Context, filter HistoryFilter) ([]Task, error)
	Counts(ctx context.Context, since time.Time) (HistoryCounts, error)
	Delete(ctx context.Context, id string) error
	// Purge remove todas as tasks que casam com o filtro (Limit é ignorado)
	// e devolve quantas foram removidas.
	Purge(ctx context.Context, filter HistoryFilter) (int, error)
}

// HistoryFilter restringe a listagem. Os zero values significam "sem filtro".
type HistoryFilter struct {
	Status string    // queued, scheduled, running, completed, failed; "" = todas
	Search string    // substring em ID ou nome
	Since  time.Time // zero = sem limite de período
	Limit  int       // 0 = DefaultHistoryLimit
}

const DefaultHistoryLimit = 100

// HistoryCounts são as contagens exibidas nos cards da UI.
// Retried conta tasks que tiveram ao menos uma retentativa.
type HistoryCounts struct {
	Queued    int
	Scheduled int
	Running   int
	Completed int
	Failed    int
	Retried   int
}
