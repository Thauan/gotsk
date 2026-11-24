package gotsk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/stretchr/testify/require"
)

func TestDictionariesHaveSameKeys(t *testing.T) {
	for key := range messages[langPT] {
		_, ok := messages[langEN][key]
		require.True(t, ok, "falta tradução em inglês para %q", key)
	}
	for key := range messages[langEN] {
		_, ok := messages[langPT][key]
		require.True(t, ok, "falta tradução em português para %q", key)
	}
}

func TestLanguageDetection(t *testing.T) {
	req := func(accept, cookie string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if accept != "" {
			r.Header.Set("Accept-Language", accept)
		}
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: langCookie, Value: cookie})
		}
		return r
	}
	require.Equal(t, langPT, langFromRequest(req("", "")), "padrão é português")
	require.Equal(t, langEN, langFromRequest(req("en-US,en;q=0.9", "")))
	require.Equal(t, langPT, langFromRequest(req("fr-FR", "")), "idioma não suportado cai no padrão")
	require.Equal(t, langEN, langFromRequest(req("pt-BR", "en")), "cookie tem prioridade sobre o navegador")
	require.Equal(t, langEN, langFromRequest(req("en", "xx")), "cookie inválido é ignorado e cai no navegador")
}

func TestPagesRenderInEnglish(t *testing.T) {
	h, _, _ := newUITest(t)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: langCookie, Value: langEN})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	body := rec.Body.String()
	require.Contains(t, body, `<html lang="en"`)
	require.Contains(t, body, "Completed")
	require.Contains(t, body, "Refresh data")
	require.NotContains(t, body, "Atualizar dados")
}

func TestLanguageSwitchSetsCookieAndReturns(t *testing.T) {
	h, _, _ := newUITest(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lang?set=en&back="+url.QueryEscape("/settings?x=1"), nil))
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Equal(t, "/settings?x=1", rec.Header().Get("Location"))
	require.Contains(t, rec.Header().Get("Set-Cookie"), langCookie+"=en")

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lang?set=en&back=https://evil.example", nil))
	require.Equal(t, "/", rec.Header().Get("Location"), "destino externo é ignorado")

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lang?set=fr", nil))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestActionMessagesFollowLanguage(t *testing.T) {
	h, _, _ := newUITest(t)
	req := httptest.NewRequest(http.MethodPost, "/settings/pause", strings.NewReader(url.Values{}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	req.AddCookie(&http.Cookie{Name: langCookie, Value: langEN})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Contains(t, rec.Header().Get("Location"), url.QueryEscape("Processing paused."))
}

func TestLogDownloadFollowsLanguage(t *testing.T) {
	h, repo, _ := newUITest(t)
	now := time.Now()
	require.NoError(t, repo.Save(context.Background(), interfaces.Task{
		ID: "task-en", Name: "job", Status: "completed", CreatedAt: now, StartedAt: now, FinishedAt: now,
		Attempts: []interfaces.Attempt{{Number: 1, StartedAt: now, FinishedAt: now, WorkerID: "w"}},
	}))
	r := httptest.NewRequest(http.MethodGet, "/tasks/task-en/log", nil)
	r.AddCookie(&http.Cookie{Name: langCookie, Value: langEN})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	body, _ := io.ReadAll(rec.Body)
	require.Contains(t, string(body), "Attempts (1)")
	require.Contains(t, string(body), "success")
}
