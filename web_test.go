package gotsk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Thauan/gotsk/history/sqlite"
	"github.com/Thauan/gotsk/interfaces"
	"github.com/stretchr/testify/require"
)

func newUITest(t *testing.T) (http.Handler, *sqlite.Repository, *Queue) {
	t.Helper()
	repo, err := sqlite.Open("file:" + filepath.Join(t.TempDir(), "gotsk.db"))
	require.NoError(t, err)
	t.Cleanup(func() { repo.Close() })

	now := time.Now()
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "task-ok", Name: "send_email", Status: "completed", CreatedAt: now}))
	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "task-bad", Name: "resize_image", Status: "failed", Retries: 3, CreatedAt: now}))
	require.NoError(t, repo.Save(ctx, interfaces.Task{ID: "task-old", Name: "send_email", Status: "completed", CreatedAt: now.Add(-10 * 24 * time.Hour)}))

	queue := NewWithStore(1, NewMemoryStore())
	queue.SetHistory(repo)
	h, err := queue.uiHandler()
	require.NoError(t, err)
	return h, repo, queue
}

func post(t *testing.T, h http.Handler, target, origin string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func get(t *testing.T, h http.Handler, target string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	body, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, string(body)
}

func TestDashboardShowsRealCounts(t *testing.T) {
	h, _, _ := newUITest(t)

	code, body := get(t, h, "/")
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, body, "task-ok")
	require.Contains(t, body, "task-bad")
	require.Contains(t, body, "task-old")
	require.Contains(t, body, "Concluída")
	require.Contains(t, body, "Falha")
	require.NotContains(t, body, "1245", "não deve haver números fixos do protótipo")
}

func TestDashboardFiltersByStatusAndPeriod(t *testing.T) {
	h, _, _ := newUITest(t)

	_, body := get(t, h, "/tasks?status=failed")
	require.Contains(t, body, "task-bad")
	require.NotContains(t, body, "task-ok")

	_, body = get(t, h, "/tasks?period=7d")
	require.Contains(t, body, "task-ok")
	require.NotContains(t, body, "task-old", "tarefa de 10 dias não entra no período de 7 dias")

	_, body = get(t, h, "/stats?period=7d")
	require.Contains(t, body, ">1<", "contagem de concluídas nos últimos 7 dias")
}

func TestUnknownStatusFallsBackToAll(t *testing.T) {
	h, _, _ := newUITest(t)
	_, body := get(t, h, "/tasks?status=<script>")
	require.Contains(t, body, "task-ok")
	require.False(t, strings.Contains(body, "<script>"))
}

func TestUIWithoutHistoryRendersEmptyState(t *testing.T) {
	queue := NewWithStore(1, NewMemoryStore())
	h, err := queue.uiHandler()
	require.NoError(t, err)

	code, body := get(t, h, "/")
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, body, "Histórico não configurado")
	require.Contains(t, body, "Nenhuma tarefa encontrada")
}

func TestUnknownPathIs404(t *testing.T) {
	h, _, _ := newUITest(t)
	code, _ := get(t, h, "/nao-existe")
	require.Equal(t, http.StatusNotFound, code)
}

func TestSettingsPageRenders(t *testing.T) {
	h, _, _ := newUITest(t)
	code, body := get(t, h, "/settings")
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, body, "Processamento")
	require.Contains(t, body, "SQLite")
}

func TestActionsRejectCrossOriginRequests(t *testing.T) {
	h, repo, queue := newUITest(t)

	rec := post(t, h, "/settings/pause", "https://evil.example", url.Values{})
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.False(t, queue.Paused())

	form := url.Values{"scope": {"all"}, "confirm": {"yes"}}
	rec = post(t, h, "/settings/history/purge", "https://evil.example", form)
	require.Equal(t, http.StatusForbidden, rec.Code)
	counts, err := repo.Counts(context.Background(), time.Time{})
	require.NoError(t, err)
	require.Equal(t, 3, counts.Completed+counts.Failed, "nada deve ter sido apagado")
}

func TestActionsRequirePOST(t *testing.T) {
	h, _, _ := newUITest(t)
	code, _ := get(t, h, "/settings/pause")
	require.Equal(t, http.StatusMethodNotAllowed, code)
}

func TestPauseAndResumeFromSettings(t *testing.T) {
	h, _, queue := newUITest(t)

	rec := post(t, h, "/settings/pause", "", url.Values{})
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.True(t, queue.Paused())

	rec = post(t, h, "/settings/resume", "", url.Values{})
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.False(t, queue.Paused())
}

func TestPurgeRequiresConfirmation(t *testing.T) {
	h, repo, _ := newUITest(t)
	ctx := context.Background()

	rec := post(t, h, "/settings/history/purge", "", url.Values{"scope": {"completed"}})
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Contains(t, rec.Header().Get("Location"), "kind=erro")

	counts, err := repo.Counts(ctx, time.Time{})
	require.NoError(t, err)
	require.Equal(t, 2, counts.Completed, "sem confirmação nada é apagado")

	rec = post(t, h, "/settings/history/purge", "", url.Values{"scope": {"completed"}, "confirm": {"yes"}})
	require.Contains(t, rec.Header().Get("Location"), "2+tasks+conclu")

	counts, err = repo.Counts(ctx, time.Time{})
	require.NoError(t, err)
	require.Equal(t, 0, counts.Completed)
	require.Equal(t, 1, counts.Failed, "falhas não são afetadas pelo escopo concluídas")
}

func TestPurgeRejectsUnknownScope(t *testing.T) {
	h, _, _ := newUITest(t)
	rec := post(t, h, "/settings/history/purge", "", url.Values{"scope": {"queued"}, "confirm": {"yes"}})
	require.Contains(t, rec.Header().Get("Location"), "kind=erro")
}

func TestTaskDetailShowsExecutionData(t *testing.T) {
	h, repo, _ := newUITest(t)
	now := time.Now()
	require.NoError(t, repo.Save(context.Background(), interfaces.Task{
		ID: "task-retry", Name: "send_email", Status: "failed", Retries: 2,
		Payload: interfaces.Payload{"to": "ana@example.com"}, Priority: 5,
		CreatedAt: now.Add(-3 * time.Second), StartedAt: now.Add(-2 * time.Second), FinishedAt: now,
		WorkerID: "worker-xyz", Error: "smtp recusou a conexão",
		Attempts: []interfaces.Attempt{
			{Number: 1, StartedAt: now.Add(-2 * time.Second), FinishedAt: now.Add(-2 * time.Second).Add(50 * time.Millisecond), WorkerID: "worker-xyz", Error: "timeout"},
			{Number: 2, StartedAt: now.Add(-1 * time.Second), FinishedAt: now, WorkerID: "worker-xyz", Error: "smtp recusou a conexão"},
		},
	}))

	code, body := get(t, h, "/tasks/task-retry")
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, body, "send_email")
	require.Contains(t, body, "worker-xyz")
	require.Contains(t, body, "timeout")
	require.Contains(t, body, "smtp recusou a conexão")
	require.Contains(t, body, "ana@example.com", "payload aparece na página")
	require.Contains(t, body, "Espera na fila")
	require.Contains(t, body, "/tasks/task-retry/requeue")
}

func TestTaskDetailNotFound(t *testing.T) {
	h, _, _ := newUITest(t)
	code, _ := get(t, h, "/tasks/nao-existe")
	require.Equal(t, http.StatusNotFound, code)
}

func TestTaskDetailEscapesHTML(t *testing.T) {
	h, repo, _ := newUITest(t)
	require.NoError(t, repo.Save(context.Background(), interfaces.Task{
		ID: "xss", Name: "job", Status: "failed", Error: "<script>alert(1)</script>", CreatedAt: time.Now(),
	}))
	_, body := get(t, h, "/tasks/xss")
	require.NotContains(t, body, "<script>alert(1)</script>")
	require.Contains(t, body, "&lt;script&gt;")
}

func TestTaskActionsRejectCrossOrigin(t *testing.T) {
	h, repo, _ := newUITest(t)
	rec := post(t, h, "/tasks/task-ok/delete", "https://evil.example", url.Values{})
	require.Equal(t, http.StatusForbidden, rec.Code)
	_, err := repo.Get(context.Background(), "task-ok")
	require.NoError(t, err, "task não deve ter sido removida")
}

func TestDeleteTaskFromDetail(t *testing.T) {
	h, repo, _ := newUITest(t)
	rec := post(t, h, "/tasks/task-ok/delete", "", url.Values{})
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Contains(t, rec.Header().Get("Location"), "/?")
	_, err := repo.Get(context.Background(), "task-ok")
	require.ErrorIs(t, err, interfaces.ErrTaskNotFound)
}

func TestRequeueTaskOnlyWorksForFailures(t *testing.T) {
	h, _, _ := newUITest(t)
	rec := post(t, h, "/tasks/task-ok/requeue", "", url.Values{})
	require.Contains(t, rec.Header().Get("Location"), "kind=erro")
}
