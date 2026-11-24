package gotsk

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/Thauan/gotsk/middlewares"
)

// Os templates são embarcados no binário: a UI não depende do diretório de
// execução.
//
//go:embed web-ui/templates
var templatesFS embed.FS

type tab struct {
	Label  string
	Href   string
	Active bool
}

// langFlag é uma bandeira do seletor de idioma.
type langFlag struct {
	Code   string
	Label  string
	Href   string
	Active bool
	Icon   template.HTML
}

// shell são os dados comuns a todas as telas: idioma, tradutor, item ativo
// da barra lateral e as bandeiras.
type shell struct {
	Lang        string
	T           translator
	Active      string
	Flags       []langFlag
	AuthEnabled bool
}

type taskView struct {
	T           translator
	CanRequeue  bool
	ID          string
	Name        string
	Status      string
	StatusLabel string
	StatusClass string
	Retries     int
	CreatedAt   string
}

type dashboardView struct {
	shell
	Flash      string
	FlashError bool
	HasHistory bool
	Counts     interfaces.HistoryCounts
	Tabs       []tab
	Periods    []tab
	Status     string
	Search     string
	Period     string
	Tasks      []taskView
}

// statusClasses são as cores do badge de cada status. O rótulo vem do
// tradutor, na chave status.<nome>.
var statusClasses = map[string]string{
	"queued":    "bg-gray-100 text-gray-700",
	"scheduled": "bg-amber-100 text-amber-800",
	"running":   "bg-blue-100 text-blue-800",
	"completed": "bg-green-100 text-green-800",
	"failed":    "bg-red-100 text-red-800",
}

var statusTabs = []struct{ value, label string }{
	{"", "tab.all"},
	{"queued", "tab.queued"},
	{"running", "tab.running"},
	{"completed", "tab.completed"},
	{"failed", "tab.failed"},
}

var periods = []struct{ value, label string }{
	{"24h", "period.24h"},
	{"7d", "period.7d"},
	{"all", "period.all"},
}

// Bandeiras simplificadas em SVG, sem dependências externas.
const flagBR template.HTML = `<svg viewBox="0 0 60 40" class="h-4 w-6 rounded-sm" aria-hidden="true"><rect width="60" height="40" fill="#009c3b"/><path d="M30 4 L56 20 L30 36 L4 20 Z" fill="#ffdf00"/><circle cx="30" cy="20" r="8" fill="#002776"/></svg>`

const flagUS template.HTML = `<svg viewBox="0 0 60 40" class="h-4 w-6 rounded-sm" aria-hidden="true"><rect width="60" height="40" fill="#fff"/><rect y="0" width="60" height="3.1" fill="#b22234"/><rect y="6.15" width="60" height="3.1" fill="#b22234"/><rect y="12.3" width="60" height="3.1" fill="#b22234"/><rect y="18.45" width="60" height="3.1" fill="#b22234"/><rect y="24.6" width="60" height="3.1" fill="#b22234"/><rect y="30.75" width="60" height="3.1" fill="#b22234"/><rect y="36.9" width="60" height="3.1" fill="#b22234"/><rect width="26" height="21.5" fill="#3c3b6e"/></svg>`

// newShell monta os dados comuns da tela para a requisição: idioma escolhido
// e links para trocar de idioma mantendo a tela atual.
func newShell(r *http.Request, active string) shell {
	lang := langFromRequest(r)
	back := url.QueryEscape(r.URL.RequestURI())
	return shell{
		Lang:   lang,
		T:      newTranslator(lang),
		Active: active,
		Flags: []langFlag{
			{Code: langPT, Label: "Português (Brasil)", Href: "/lang?set=" + langPT + "&back=" + back, Active: lang == langPT, Icon: flagBR},
			{Code: langEN, Label: "English", Href: "/lang?set=" + langEN + "&back=" + back, Active: lang == langEN, Icon: flagUS},
		},
	}
}

// statusLabel devolve o rótulo traduzido e a cor do status.
func statusLabel(tr translator, status string) (string, string) {
	cls, ok := statusClasses[status]
	if !ok {
		return status, "bg-gray-100 text-gray-700"
	}
	return tr("status." + status), cls
}

// uiFilters lê e valida os parâmetros da URL. Valores desconhecidos viram o padrão.
func uiFilters(r *http.Request) (status, search, period string) {
	q := r.URL.Query()
	status = q.Get("status")
	if _, ok := statusClasses[status]; !ok {
		status = ""
	}
	period = q.Get("period")
	if period != "24h" && period != "7d" {
		period = "all"
	}
	return status, q.Get("q"), period
}

func periodSince(period string) time.Time {
	switch period {
	case "24h":
		return time.Now().Add(-24 * time.Hour)
	case "7d":
		return time.Now().Add(-7 * 24 * time.Hour)
	}
	return time.Time{}
}

// href monta um link para a própria UI trocando um parâmetro e mantendo os outros.
func href(status, search, period string) string {
	v := url.Values{}
	if status != "" {
		v.Set("status", status)
	}
	if search != "" {
		v.Set("q", search)
	}
	if period != "" && period != "all" {
		v.Set("period", period)
	}
	if len(v) == 0 {
		return "/"
	}
	return "/?" + v.Encode()
}

func (q *Queue) uiTemplates() (*template.Template, error) {
	return template.ParseFS(templatesFS,
		"web-ui/templates/*.html",
		"web-ui/templates/partials/*.html",
	)
}

func (q *Queue) buildView(r *http.Request) (dashboardView, error) {
	status, search, period := uiFilters(r)
	sh := newShell(r, "tasks")
	sh.AuthEnabled = q.authEnabled()
	view := dashboardView{
		shell:      sh,
		Flash:      r.URL.Query().Get("msg"),
		FlashError: r.URL.Query().Get("kind") == "erro",
		HasHistory: q.repo != nil,
		Status:     status,
		Search:     search,
		Period:     period,
	}

	for _, t := range statusTabs {
		view.Tabs = append(view.Tabs, tab{
			Label:  sh.T(t.label),
			Href:   href(t.value, search, period),
			Active: t.value == status,
		})
	}
	for _, p := range periods {
		view.Periods = append(view.Periods, tab{
			Label:  sh.T(p.label),
			Href:   href(status, search, p.value),
			Active: p.value == period,
		})
	}

	if q.repo == nil {
		return view, nil
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	since := periodSince(period)

	counts, err := q.repo.Counts(ctx, since)
	if err != nil {
		return view, fmt.Errorf("counting tasks: %w", err)
	}
	view.Counts = counts

	tasks, err := q.repo.List(ctx, interfaces.HistoryFilter{Status: status, Search: search, Since: since})
	if err != nil {
		return view, fmt.Errorf("listing tasks: %w", err)
	}
	for _, t := range tasks {
		view.Tasks = append(view.Tasks, toTaskView(t, sh.T))
	}
	return view, nil
}

func toTaskView(t interfaces.Task, tr translator) taskView {
	label, class := statusLabel(tr, t.Status)
	return taskView{
		T:           tr,
		CanRequeue:  t.Status == "failed",
		ID:          t.ID,
		Name:        t.Name,
		Status:      t.Status,
		StatusLabel: label,
		StatusClass: class,
		Retries:     t.Retries,
		CreatedAt:   t.CreatedAt.Format(tr("time.short")),
	}
}

func (q *Queue) uiHandler() (http.Handler, error) {
	tmpl, err := q.uiTemplates()
	if err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	render := func(w http.ResponseWriter, name string, data any) {
		if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
			log.Printf("error rendering %s: %v", name, err)
		}
	}
	fail := func(w http.ResponseWriter, err error) {
		log.Printf("UI error: %v", err)
		http.Error(w, "error querying history", http.StatusInternalServerError)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		view, err := q.buildView(r)
		if err != nil {
			fail(w, err)
			return
		}
		render(w, "task-dashboard.html", view)
	})

	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		view, err := q.buildView(r)
		if err != nil {
			fail(w, err)
			return
		}
		render(w, "stats", view)
	})

	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		view, err := q.buildView(r)
		if err != nil {
			fail(w, err)
			return
		}
		render(w, "task-table", view)
	})

	mux.HandleFunc("/events", q.streamEvents)
	mux.HandleFunc("/login", q.loginHandler)
	mux.HandleFunc("/logout", q.logoutHandler)

	// /lang grava a escolha de idioma num cookie e volta para a tela de origem.
	mux.HandleFunc("/lang", func(w http.ResponseWriter, r *http.Request) {
		set := r.URL.Query().Get("set")
		if !supportedLang(set) {
			http.Error(w, "language not supported", http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     langCookie,
			Value:    set,
			Path:     "/",
			MaxAge:   365 * 24 * 60 * 60,
			SameSite: http.SameSiteLaxMode,
		})
		back := r.URL.Query().Get("back")
		if !strings.HasPrefix(back, "/") || strings.HasPrefix(back, "//") {
			back = "/"
		}
		http.Redirect(w, r, back, http.StatusSeeOther)
	})

	mux.HandleFunc("/tasks/{id}", q.taskDetail)
	mux.HandleFunc("GET /tasks/{id}/log", q.taskLog)
	mux.HandleFunc("/tasks/{id}/requeue", q.actionBack(backToDashboard, func(w http.ResponseWriter, r *http.Request) (string, bool) {
		tr := newTranslator(langFromRequest(r))
		if q.repo == nil {
			return tr("err.noHistory"), true
		}
		id := r.PathValue("id")
		if err := q.RequeueTask(r.Context(), id); err != nil {
			if errors.Is(err, interfaces.ErrTaskNotFound) {
				return tr("msg.taskNotFound"), true
			}
			if errors.Is(err, ErrTaskNotFailed) {
				return tr("msg.taskNotFailed"), true
			}
			return tr("msg.error", err), true
		}
		return tr("msg.taskRequeued"), false
	}))
	mux.HandleFunc("/tasks/{id}/delete", q.actionBack(backToDashboard, func(w http.ResponseWriter, r *http.Request) (string, bool) {
		tr := newTranslator(langFromRequest(r))
		if q.repo == nil {
			return tr("err.noHistory"), true
		}
		if err := q.repo.Delete(r.Context(), r.PathValue("id")); err != nil {
			return tr("msg.taskDeleteError"), true
		}
		return tr("msg.taskRemoved"), false
	}))

	mux.HandleFunc("/settings", func(w http.ResponseWriter, r *http.Request) {
		view, err := q.buildSettings(r)
		if err != nil {
			fail(w, err)
			return
		}
		render(w, "settings.html", view)
	})
	mux.HandleFunc("/settings/pause", q.action(func(w http.ResponseWriter, r *http.Request) (string, bool) {
		q.Pause()
		return newTranslator(langFromRequest(r))("msg.paused"), false
	}))
	mux.HandleFunc("/settings/resume", q.action(func(w http.ResponseWriter, r *http.Request) (string, bool) {
		q.Resume()
		return newTranslator(langFromRequest(r))("msg.resumed"), false
	}))
	mux.HandleFunc("/settings/dead/requeue", q.action(func(w http.ResponseWriter, r *http.Request) (string, bool) {
		tr := newTranslator(langFromRequest(r))
		n, err := q.RequeueDead(r.Context())
		if err != nil {
			if errors.Is(err, ErrNoDeadLetter) {
				return tr("err.noDeadLetter"), true
			}
			log.Printf("error requeuing dead-letter: %v", err)
			return tr("msg.deadRequeueError"), true
		}
		return tr("msg.deadRequeued", n), false
	}))
	mux.HandleFunc("/settings/history/purge", q.action(func(w http.ResponseWriter, r *http.Request) (string, bool) {
		tr := newTranslator(langFromRequest(r))
		if q.repo == nil {
			return tr("err.noHistory"), true
		}
		if r.FormValue("confirm") != "yes" {
			return tr("msg.purgeCancelled"), true
		}
		var filter interfaces.HistoryFilter
		var scope string
		switch r.FormValue("scope") {
		case "completed":
			filter.Status, scope = "completed", tr("scope.completed")
		case "failed":
			filter.Status, scope = "failed", tr("scope.failed")
		case "all":
			scope = tr("scope.all")
		default:
			return tr("msg.purgeScope"), true
		}
		n, err := q.repo.Purge(r.Context(), filter)
		if err != nil {
			log.Printf("error clearing history: %v", err)
			return tr("msg.purgeError"), true
		}
		return tr("msg.purgeDone", n, scope), false
	}))
	mux.HandleFunc("/settings/failed/requeue", q.action(func(w http.ResponseWriter, r *http.Request) (string, bool) {
		tr := newTranslator(langFromRequest(r))
		requeued, skipped, err := q.RequeueFailed(r.Context())
		if err != nil {
			if errors.Is(err, ErrNoHistory) {
				return tr("err.noHistory"), true
			}
			log.Printf("error requeuing failures: %v", err)
			return tr("msg.requeueError"), true
		}
		msg := tr("msg.requeued", requeued)
		if skipped > 0 {
			msg += tr("msg.requeuedSkipped", skipped)
		}
		return msg, skipped > 0
	}))

	return q.withAuth(mux), nil
}

// action envolve uma ação que altera estado: aceita só POST de mesma origem
// e volta para /settings com uma mensagem (padrão PRG, sem reenvio do form).
// Sem token CSRF, a checagem de origem impede que outra página no navegador
// dispare a ação.
func (q *Queue) action(fn func(w http.ResponseWriter, r *http.Request) (msg string, isErr bool)) http.HandlerFunc {
	return q.actionTo("/settings", fn)
}

// actionTo é action redirecionando para base com a mensagem.
func (q *Queue) actionTo(base string, fn func(w http.ResponseWriter, r *http.Request) (msg string, isErr bool)) http.HandlerFunc {
	return q.actionBack(func(*http.Request) string { return base }, fn)
}

// actionBack é action com destino calculado pela requisição.
func (q *Queue) actionBack(base func(*http.Request) string, fn func(w http.ResponseWriter, r *http.Request) (msg string, isErr bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "use POST", http.StatusMethodNotAllowed)
			return
		}
		if !sameOrigin(r) {
			http.Error(w, newTranslator(langFromRequest(r))("msg.origin"), http.StatusForbidden)
			return
		}
		msg, isErr := fn(w, r)
		kind := "ok"
		if isErr {
			kind = "erro"
		}
		target := base(r)
		sep := "?"
		if strings.Contains(target, "?") {
			sep = "&"
		}
		target += sep + url.Values{"msg": {msg}, "kind": {kind}}.Encode()
		http.Redirect(w, r, target, http.StatusSeeOther)
	}
}

// backToDashboard volta para o painel com os filtros de onde a ação partiu,
// quando a origem é o painel. Em outras telas, volta para o painel sem filtros.
func backToDashboard(r *http.Request) string {
	if !sameOrigin(r) {
		return "/"
	}
	ref, err := url.Parse(r.Header.Get("Referer"))
	if err != nil || ref.Path != "/" {
		return "/"
	}
	return "/?" + ref.RawQuery
}

func sameOrigin(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin" || site == "none"
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}

type settingsView struct {
	shell
	DeadConfigured bool
	DeadCount      int
	DeadCountOK    bool
	Flash          string
	FlashError     bool
	Paused         bool
	Workers        int
	Uptime         string
	HasHistory     bool
	Backend        string
	Counts         interfaces.HistoryCounts
}

func (q *Queue) buildSettings(r *http.Request) (settingsView, error) {
	view := settingsView{
		shell:      withAuthFlag(newShell(r, "settings"), q.authEnabled()),
		Flash:      r.URL.Query().Get("msg"),
		FlashError: r.URL.Query().Get("kind") == "erro",
		Paused:     q.Paused(),
		Workers:    q.GetWorkers(),
		Uptime:     q.Uptime().Round(time.Second).String(),
		HasHistory: q.repo != nil,
	}
	view.DeadConfigured = q.DeadLetterConfigured()
	if view.DeadConfigured {
		n, ok, err := q.DeadLetterSize(r.Context())
		if err != nil {
			log.Printf("error counting dead-letter: %v", err)
		}
		view.DeadCount, view.DeadCountOK = n, ok && err == nil
	}
	if q.repo == nil {
		return view, nil
	}
	if named, ok := q.repo.(interface{ Name() string }); ok {
		view.Backend = named.Name()
	}
	counts, err := q.repo.Counts(r.Context(), time.Time{})
	if err != nil {
		return view, fmt.Errorf("counting tasks: %w", err)
	}
	view.Counts = counts
	return view, nil
}

type attemptView struct {
	Number   int
	Started  string
	Duration string
	Worker   string
	Error    string
	Failed   bool
}

type taskDetailView struct {
	shell
	Flash       string
	FlashError  bool
	ID          string
	Name        string
	Status      string
	StatusLabel string
	StatusClass string
	Priority    int
	Retries     int
	Worker      string
	CreatedAt   string
	ScheduledAt string
	StartedAt   string
	FinishedAt  string
	QueueWait   string
	Duration    string
	Error       string
	Payload     string
	Attempts    []attemptView
	CanRequeue  bool
}

func fmtTime(t time.Time, layout string) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format(layout)
}

func fmtSpan(from, to time.Time) string {
	if from.IsZero() || to.IsZero() {
		return "—"
	}
	return to.Sub(from).Round(time.Millisecond).String()
}

func toTaskDetail(t interfaces.Task, tr translator) taskDetailView {
	label, class := statusLabel(tr, t.Status)
	payload, err := json.MarshalIndent(t.Payload, "", "  ")
	if err != nil {
		payload = []byte(fmt.Sprint(t.Payload))
	}
	layout := tr("time.long")
	view := taskDetailView{
		ID:          t.ID,
		Name:        t.Name,
		Status:      t.Status,
		StatusLabel: label,
		StatusClass: class,
		Priority:    t.Priority,
		Retries:     t.Retries,
		Worker:      t.WorkerID,
		CreatedAt:   fmtTime(t.CreatedAt, layout),
		ScheduledAt: fmtTime(t.ScheduledAt, layout),
		StartedAt:   fmtTime(t.StartedAt, layout),
		FinishedAt:  fmtTime(t.FinishedAt, layout),
		QueueWait:   fmtSpan(t.CreatedAt, t.StartedAt),
		Duration:    fmtSpan(t.StartedAt, t.FinishedAt),
		Error:       t.Error,
		Payload:     string(payload),
		CanRequeue:  t.Status == "failed",
	}
	for _, a := range t.Attempts {
		view.Attempts = append(view.Attempts, attemptView{
			Number:   a.Number,
			Started:  fmtTime(a.StartedAt, layout),
			Duration: fmtSpan(a.StartedAt, a.FinishedAt),
			Worker:   a.WorkerID,
			Error:    a.Error,
			Failed:   a.Error != "",
		})
	}
	return view
}

func (q *Queue) taskDetail(w http.ResponseWriter, r *http.Request) {
	if q.repo == nil {
		http.Error(w, ErrNoHistory.Error(), http.StatusNotFound)
		return
	}
	t, err := q.repo.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, interfaces.ErrTaskNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("error loading task: %v", err)
		http.Error(w, "error querying history", http.StatusInternalServerError)
		return
	}

	sh := newShell(r, "tasks")
	sh.AuthEnabled = q.authEnabled()
	view := toTaskDetail(t, sh.T)
	view.shell = sh
	view.Flash = r.URL.Query().Get("msg")
	view.FlashError = r.URL.Query().Get("kind") == "erro"

	tmpl, err := q.uiTemplates()
	if err != nil {
		http.Error(w, "error loading templates", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "task-detail.html", view); err != nil {
		log.Printf("error rendering task-detail: %v", err)
	}
}

// taskLog devolve um arquivo de texto com os dados da execução, para anexar
// a um chamado ou comparar execuções.
func (q *Queue) taskLog(w http.ResponseWriter, r *http.Request) {
	tr := newTranslator(langFromRequest(r))
	if q.repo == nil {
		http.Error(w, tr("err.noHistory"), http.StatusNotFound)
		return
	}
	id := r.PathValue("id")
	t, err := q.repo.Get(r.Context(), id)
	if errors.Is(err, interfaces.ErrTaskNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("error generating log: %v", err)
		http.Error(w, "error querying history", http.StatusInternalServerError)
		return
	}

	v := toTaskDetail(t, tr)
	var b strings.Builder
	line := func(key, value string) { fmt.Fprintf(&b, "%-15s %s\n", tr(key)+":", value) }
	line("log.task", v.ID)
	line("log.name", v.Name)
	line("log.status", v.Status)
	line("log.priority", fmt.Sprint(v.Priority))
	line("log.created", v.CreatedAt)
	line("log.scheduled", v.ScheduledAt)
	line("log.started", v.StartedAt)
	line("log.finished", v.FinishedAt)
	line("log.wait", v.QueueWait)
	line("log.duration", v.Duration)
	line("log.worker", v.Worker)
	fmt.Fprintf(&b, "\n%s:\n%s\n", tr("log.payload"), v.Payload)
	fmt.Fprintf(&b, "\n%s\n", tr("log.attempts", len(v.Attempts)))
	for _, a := range v.Attempts {
		result := tr("log.ok")
		if a.Failed {
			result = tr("log.error", a.Error)
		}
		fmt.Fprintf(&b, "%s\n", tr("log.attemptLine", a.Number, a.Started, a.Duration, a.Worker, result))
	}
	if v.Error != "" {
		fmt.Fprintf(&b, "\n%s\n", tr("log.lastError", v.Error))
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.log"`, id))
	io.WriteString(w, b.String())
}

// ServeUI sobe o painel em addr e para quando ctx é cancelado.
func (q *Queue) ServeUI(addr string, ctx context.Context) {
	handler, err := q.uiHandler()
	if err != nil {
		log.Fatalf("error starting UI: %v", err)
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: middlewares.HTTPLoggingMiddleware(log.New(os.Stdout, "", log.LstdFlags))(handler),
	}

	go func() {
		<-ctx.Done()
		log.Println("🛑 Shutting down UI server...")
		srv.Shutdown(context.Background())
	}()

	if !q.authEnabled() {
		log.Printf("⚠️ UI without authentication: call SetUIToken to protect it")
	}
	go func() {
		log.Printf("🌐 Serving UI at http://%s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("error starting UI: %v", err)
		}
	}()
}

// streamEvents envia uma mensagem SSE por mudança de estado. O payload é só
// o ID e o status: o navegador recarrega a tabela e os cards a partir do
// histórico, então não precisa montar HTML aqui.
func (q *Queue) streamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := q.Subscribe()
	defer q.unregisterSSEClient(ch)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case task := <-ch:
			data, err := json.Marshal(map[string]string{"id": task.ID, "status": task.Status})
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: task\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func withAuthFlag(sh shell, enabled bool) shell {
	sh.AuthEnabled = enabled
	return sh
}
