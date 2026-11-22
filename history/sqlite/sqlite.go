// Package sqlite implementa interfaces.HistoryRepository sobre SQLite.
// Usa o driver modernc.org/sqlite, que é puro Go (sem cgo).
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
	id           TEXT PRIMARY KEY,
	name         TEXT NOT NULL,
	status       TEXT NOT NULL,
	payload      TEXT NOT NULL,
	retries      INTEGER NOT NULL DEFAULT 0,
	priority     INTEGER NOT NULL DEFAULT 0,
	scheduled_at INTEGER NOT NULL DEFAULT 0,
	created_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS tasks_status_created ON tasks (status, created_at);
CREATE INDEX IF NOT EXISTS tasks_created ON tasks (created_at);
`

// added são as colunas acrescentadas depois da primeira versão do schema.
// Bancos criados antes ganham essas colunas na abertura.
var added = []struct{ name, ddl string }{
	{"started_at", "INTEGER NOT NULL DEFAULT 0"},
	{"finished_at", "INTEGER NOT NULL DEFAULT 0"},
	{"worker_id", "TEXT NOT NULL DEFAULT ''"},
	{"error", "TEXT NOT NULL DEFAULT ''"},
	{"attempts", "TEXT NOT NULL DEFAULT '[]'"},
}

type Repository struct {
	db *sql.DB
}

// Open abre (ou cria) o banco em dsn, por exemplo "file:gotsk.db".
func Open(dsn string) (*Repository, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite aceita um escritor por vez; uma conexão evita SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("criando schema: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Repository{db: db}, nil
}

func migrate(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(tasks)`)
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		existing[name] = true
	}
	rows.Close()

	for _, c := range added {
		if existing[c.name] {
			continue
		}
		if _, err := db.Exec(fmt.Sprintf("ALTER TABLE tasks ADD COLUMN %s %s", c.name, c.ddl)); err != nil {
			return fmt.Errorf("migrando coluna %s: %w", c.name, err)
		}
	}
	return nil
}

func (r *Repository) Close() error { return r.db.Close() }

const selectColumns = `id, name, status, payload, retries, priority, scheduled_at, created_at,
	started_at, finished_at, worker_id, error, attempts`

func (r *Repository) Save(ctx context.Context, task interfaces.Task) error {
	payload, err := json.Marshal(task.Payload)
	if err != nil {
		return err
	}
	attempts, err := json.Marshal(nonNilAttempts(task.Attempts))
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO tasks (id, name, status, payload, retries, priority, scheduled_at, created_at,
			started_at, finished_at, worker_id, error, attempts)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			status = excluded.status,
			payload = excluded.payload,
			retries = excluded.retries,
			priority = excluded.priority,
			scheduled_at = excluded.scheduled_at,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			worker_id = excluded.worker_id,
			error = excluded.error,
			attempts = excluded.attempts`,
		task.ID, task.Name, task.Status, string(payload), task.Retries,
		task.Priority, toUnixNano(task.ScheduledAt), toUnixNano(task.CreatedAt),
		toUnixNano(task.StartedAt), toUnixNano(task.FinishedAt), task.WorkerID, task.Error, string(attempts),
	)
	return err
}

func nonNilAttempts(a []interfaces.Attempt) []interfaces.Attempt {
	if a == nil {
		return []interfaces.Attempt{}
	}
	return a
}

func (r *Repository) Get(ctx context.Context, id string) (interfaces.Task, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+selectColumns+` FROM tasks WHERE id = ?`, id)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return t, interfaces.ErrTaskNotFound
	}
	return t, err
}

func (r *Repository) List(ctx context.Context, f interfaces.HistoryFilter) ([]interfaces.Task, error) {
	where, args := whereClause(f)
	limit := f.Limit
	if limit <= 0 {
		limit = interfaces.DefaultHistoryLimit
	}
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx,
		`SELECT `+selectColumns+` FROM tasks`+where+` ORDER BY created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []interfaces.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *Repository) Counts(ctx context.Context, since time.Time) (interfaces.HistoryCounts, error) {
	var c interfaces.HistoryCounts
	rows, err := r.db.QueryContext(ctx, `
		SELECT status, COUNT(*) FROM tasks WHERE created_at >= ? GROUP BY status`,
		toUnixNano(since))
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return c, err
		}
		switch status {
		case "queued":
			c.Queued = n
		case "scheduled":
			c.Scheduled = n
		case "running":
			c.Running = n
		case "completed":
			c.Completed = n
		case "failed":
			c.Failed = n
		}
	}
	if err := rows.Err(); err != nil {
		return c, err
	}

	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks WHERE created_at >= ? AND retries > 0`,
		toUnixNano(since)).Scan(&c.Retried)
	return c, err
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	return err
}

func (r *Repository) Purge(ctx context.Context, f interfaces.HistoryFilter) (int, error) {
	where, args := whereClause(f)
	res, err := r.db.ExecContext(ctx, `DELETE FROM tasks`+where, args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// whereClause monta o WHERE de List e Purge a partir do mesmo filtro.
func whereClause(f interfaces.HistoryFilter) (string, []any) {
	var where []string
	var args []any
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.Search != "" {
		where = append(where, "(id LIKE ? OR name LIKE ?)")
		like := "%" + f.Search + "%"
		args = append(args, like, like)
	}
	if !f.Since.IsZero() {
		where = append(where, "created_at >= ?")
		args = append(args, toUnixNano(f.Since))
	}
	if len(where) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTask(s scanner) (interfaces.Task, error) {
	var t interfaces.Task
	var payload, attempts string
	var scheduled, created, started, finished int64
	if err := s.Scan(&t.ID, &t.Name, &t.Status, &payload, &t.Retries, &t.Priority, &scheduled, &created,
		&started, &finished, &t.WorkerID, &t.Error, &attempts); err != nil {
		return t, err
	}
	if err := json.Unmarshal([]byte(payload), &t.Payload); err != nil {
		return t, err
	}
	if err := json.Unmarshal([]byte(attempts), &t.Attempts); err != nil {
		return t, err
	}
	t.ScheduledAt = fromUnixNano(scheduled)
	t.CreatedAt = fromUnixNano(created)
	t.StartedAt = fromUnixNano(started)
	t.FinishedAt = fromUnixNano(finished)
	return t, nil
}

// toUnixNano trata o zero value como 0, para que "sem período" seja um
// intervalo que inclui tudo.
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
func (r *Repository) Name() string { return "SQLite" }
