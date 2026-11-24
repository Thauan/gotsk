package gotsk

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"net/url"
	"strings"
)

const sessionCookie = "gotsk_session"

// SetUIToken protege o painel com um token de acesso. Com token vazio, o
// painel fica aberto. O token é usado uma vez, no login, ou por requisições
// com o cabeçalho "Authorization: Bearer <token>". O navegador recebe uma
// sessão aleatória em cookie, nunca o token. Trocar o token ou reiniciar o
// processo encerra as sessões.
func (q *Queue) SetUIToken(token string) {
	q.uiToken = token
	q.uiSession = ""
	if token != "" {
		q.uiSession = randomHex(32)
	}
}

func (q *Queue) authEnabled() bool { return q.uiToken != "" }

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("gotsk: sem fonte de aleatoriedade: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// secureEqual compara em tempo constante, para não vazar o token por tempo de resposta.
func secureEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (q *Queue) authorized(r *http.Request) bool {
	if !q.authEnabled() {
		return true
	}
	if c, err := r.Cookie(sessionCookie); err == nil && secureEqual(c.Value, q.uiSession) {
		return true
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return secureEqual(strings.TrimPrefix(h, "Bearer "), q.uiToken)
	}
	return false
}

// withAuth exige autenticação em tudo, exceto login, logout e troca de idioma.
// Páginas abertas pelo navegador vão para o login; chamadas em segundo plano
// (atualizações, SSE, ações) recebem 401, para que o HTML do login nunca seja
// inserido na tela.
func (q *Queue) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login", "/logout", "/lang":
			next.ServeHTTP(w, r)
			return
		}
		if q.authorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Sec-Fetch-Dest") == "document" {
			http.Redirect(w, r, "/login?"+url.Values{"back": {r.URL.RequestURI()}}.Encode(), http.StatusSeeOther)
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="gotsk"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

type loginView struct {
	Lang  string
	T     translator
	Error bool
	Back  string
}

// sanitizeBack aceita só caminhos locais, para não redirecionar para outro site.
func sanitizeBack(back string) string {
	if strings.HasPrefix(back, "/") && !strings.HasPrefix(back, "//") {
		return back
	}
	return "/"
}

func (q *Queue) renderLogin(w http.ResponseWriter, r *http.Request, status int, back string, failed bool) {
	tmpl, err := q.uiTemplates()
	if err != nil {
		http.Error(w, "error loading templates", http.StatusInternalServerError)
		return
	}
	sh := newShell(r, "")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.ExecuteTemplate(w, "login.html", loginView{Lang: sh.Lang, T: sh.T, Error: failed, Back: back}); err != nil {
		log.Printf("error rendering login: %v", err)
	}
}

func (q *Queue) loginHandler(w http.ResponseWriter, r *http.Request) {
	back := sanitizeBack(r.URL.Query().Get("back"))
	if r.Method == http.MethodGet {
		q.renderLogin(w, r, http.StatusOK, back, false)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "use GET or POST", http.StatusMethodNotAllowed)
		return
	}
	if !sameOrigin(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	back = sanitizeBack(r.FormValue("back"))
	if !q.authEnabled() || !secureEqual(r.FormValue("token"), q.uiToken) {
		q.renderLogin(w, r, http.StatusUnauthorized, back, true)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    q.uiSession,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
	})
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (q *Queue) logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	if !sameOrigin(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
