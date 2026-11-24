package gotsk

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newAuthUI(t *testing.T, token string) http.Handler {
	t.Helper()
	queue := NewWithStore(1, NewMemoryStore())
	queue.SetHistory(newMemHistory())
	queue.SetUIToken(token)
	h, err := queue.uiHandler()
	require.NoError(t, err)
	return h
}

func serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func sessionOf(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("cookie de sessão não foi definido")
	return nil
}

func TestPanelOpenWithoutToken(t *testing.T) {
	h := newAuthUI(t, "")
	rec := serve(h, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestPageRedirectsToLoginWithoutSession(t *testing.T) {
	h := newAuthUI(t, "segredo")
	r := httptest.NewRequest(http.MethodGet, "/settings", nil)
	r.Header.Set("Sec-Fetch-Dest", "document")
	rec := serve(h, r)
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.True(t, strings.HasPrefix(rec.Header().Get("Location"), "/login?back="))
}

func TestBackgroundRequestsGet401WithoutSession(t *testing.T) {
	h := newAuthUI(t, "segredo")
	for _, path := range []string{"/tasks", "/stats", "/events"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("Sec-Fetch-Dest", "empty")
		rec := serve(h, r)
		require.Equal(t, http.StatusUnauthorized, rec.Code, path)
		require.NotContains(t, rec.Body.String(), "<html", "HTML do login não pode vazar para atualizações")
	}
}

func TestLoginRejectsWrongToken(t *testing.T) {
	h := newAuthUI(t, "segredo")
	form := url.Values{"token": {"errado"}, "back": {"/"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "http://example.com")
	rec := serve(h, r)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "Token inválido.")
	require.Empty(t, rec.Result().Cookies())
}

func TestLoginWithTokenSetsSessionCookie(t *testing.T) {
	h := newAuthUI(t, "segredo")
	form := url.Values{"token": {"segredo"}, "back": {"/settings"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "http://example.com")
	rec := serve(h, r)
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Equal(t, "/settings", rec.Header().Get("Location"))

	c := sessionOf(t, rec)
	require.True(t, c.HttpOnly)
	require.Equal(t, http.SameSiteStrictMode, c.SameSite)
	require.NotEqual(t, "segredo", c.Value, "o token não pode ir para o navegador")

	again := httptest.NewRequest(http.MethodGet, "/", nil)
	again.AddCookie(c)
	require.Equal(t, http.StatusOK, serve(h, again).Code)
}

func TestLoginBackRejectsExternalTarget(t *testing.T) {
	h := newAuthUI(t, "segredo")
	form := url.Values{"token": {"segredo"}, "back": {"https://evil.example"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "http://example.com")
	rec := serve(h, r)
	require.Equal(t, "/", rec.Header().Get("Location"))
}

func TestBearerTokenAuthorizesApiRequests(t *testing.T) {
	h := newAuthUI(t, "segredo")
	r := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	r.Header.Set("Authorization", "Bearer segredo")
	require.Equal(t, http.StatusOK, serve(h, r).Code)

	bad := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	bad.Header.Set("Authorization", "Bearer errado")
	require.Equal(t, http.StatusUnauthorized, serve(h, bad).Code)
}

func TestForgedCookieIsRejected(t *testing.T) {
	h := newAuthUI(t, "segredo")
	r := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "segredo"})
	require.Equal(t, http.StatusUnauthorized, serve(h, r).Code, "o token não vale como cookie")
}

func TestLogoutClearsSession(t *testing.T) {
	h := newAuthUI(t, "segredo")
	r := httptest.NewRequest(http.MethodPost, "/logout", nil)
	r.Header.Set("Origin", "http://example.com")
	rec := serve(h, r)
	require.Equal(t, http.StatusSeeOther, rec.Code)
	c := sessionOf(t, rec)
	require.Less(t, c.MaxAge, 0)
}

func TestActionsStillRejectCrossOriginWithSession(t *testing.T) {
	h := newAuthUI(t, "segredo")
	form := url.Values{"token": {"segredo"}}
	login := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	login.Header.Set("Origin", "http://example.com")
	c := sessionOf(t, serve(h, login))

	r := httptest.NewRequest(http.MethodPost, "/settings/pause", nil)
	r.AddCookie(c)
	r.Header.Set("Origin", "https://evil.example")
	require.Equal(t, http.StatusForbidden, serve(h, r).Code)
}

func TestLoginPageRenders(t *testing.T) {
	h := newAuthUI(t, "segredo")
	rec := serve(h, httptest.NewRequest(http.MethodGet, "/login?back=/settings", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `name="token"`)
}
