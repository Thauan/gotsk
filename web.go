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

type taskView struct {
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

var statusLabels = map[string][2]string{
	"queued":    {"Pendente", "bg-gray-100 text-gray-700"},
	"scheduled": {"Agendada", "bg-amber-100 text-amber-800"},
	"running":   {"Processando", "bg-blue-100 text-blue-800"},
	"completed": {"Concluída", "bg-green-100 text-green-800"},
	"failed":    {"Falha", "bg-red-100 text-red-800"},
}

var statusTabs = []struct{ value, label string }{
	{"", "Todas"},
	{"queued", "Pendentes"},
	{"running", "Processando"},
	{"completed", "Concluídas"},
	{"failed", "Falhas"},
}

var periods = []struct{ value, label string }{
	{"24h", "Últimas 24h"},
	{"7d", "Últimos 7 dias"},
	{"all", "Todo o período"},
}

// uiFilters lê e valida os parâmetros da URL. Valores desconhecidos viram o padrão.
func uiFilters(r *http.Request) (status, search, period string) {
	q := r.URL.Query()
	status = q.Get("status")
	if _, ok := statusLabels[status]; !ok {
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
	view := dashboardView{
		Flash:      r.URL.Query().Get("msg"),
		FlashError: r.URL.Query().Get("kind") == "erro",
		HasHistory: q.repo != nil,
		Status:     status,
		Search:     search,
		Period:     period,
	}

	for _, t := range statusTabs {
		view.Tabs = append(view.Tabs, tab{
			Label:  t.label,
			Href:   href(t.value, search, period),
			Active: t.value == status,
		})
	}
	for _, p := range periods {
		view.Periods = append(view.Periods, tab{
			Label:  p.label,
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
		return view, fmt.Errorf("contando tasks: %w", err)
	}
	view.Counts = counts

	tasks, err := q.repo.List(ctx, interfaces.HistoryFilter{Status: status, Search: search, Since: since})
	if err != nil {
		return view, fmt.Errorf("listando tasks: %w", err)
	}
	for _, t := range tasks {
		view.Tasks = append(view.Tasks, toTaskView(t))
	}
	return view, nil
}

func toTaskView(t interfaces.Task) taskView {
	label := statusLabels[t.Status]
	if label[0] == "" {
		label = [2]string{t.Status, "bg-gray-100 text-gray-700"}
	}
	return taskView{
		CanRequeue:  t.Status == "failed",
		ID:          t.ID,
		Name:        t.Name,
		Status:      t.Status,
		StatusLabel: label[0],
		StatusClass: label[1],
		Retries:     t.Retries,
		CreatedAt:   t.CreatedAt.Format("02/01 15:04:05"),
	}
}

func (q *Queue) uiHandler() (http.Handler, error) {
	tmpl, err := q.uiTemplates()
	if err != nil {
		return nil, fmt.Errorf("carregando templates: %w", err)
	}

	render := func(w http.ResponseWriter, name string, data any) {
		if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
			log.Printf("erro ao renderizar %s: %v", name, err)
		}
	}
	fail := func(w http.ResponseWriter, err error) {
		log.Printf("erro na UI: %v", err)
		http.Error(w, "erro ao consultar o histórico", http.StatusInternalServerError)
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
		render(w, "stats", view.Counts)
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

	mux.HandleFunc("/tasks/{id}", q.taskDetail)
	mux.HandleFunc("GET /tasks/{id}/log", q.taskLog)
	mux.HandleFunc("/tasks/{id}/requeue", q.actionBack(backToDashboard, func(w http.ResponseWriter, r *http.Request) (string, bool) {
		if q.repo == nil {
			return ErrNoHistory.Error(), true
		}
		id := r.PathValue("id")
		if err := q.RequeueTask(r.Context(), id); err != nil {
			if errors.Is(err, interfaces.ErrTaskNotFound) {
				return "Task não encontrada.", true
			}
			return err.Error(), true
		}
		return "Task reenfileirada.", false
	}))
	mux.HandleFunc("/tasks/{id}/delete", q.actionBack(backToDashboard, func(w http.ResponseWriter, r *http.Request) (string, bool) {
		if q.repo == nil {
			return ErrNoHistory.Error(), true
		}
		if err := q.repo.Delete(r.Context(), r.PathValue("id")); err != nil {
			return "Erro ao excluir a task.", true
		}
		return "Task removida do histórico.", false
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
		return "Processamento pausado. As tasks em execução vão terminar; as pendentes ficam esperando.", false
	}))
	mux.HandleFunc("/settings/resume", q.action(func(w http.ResponseWriter, r *http.Request) (string, bool) {
		q.Resume()
		return "Processamento retomado.", false
	}))
	mux.HandleFunc("/settings/history/purge", q.action(func(w http.ResponseWriter, r *http.Request) (string, bool) {
		if q.repo == nil {
			return ErrNoHistory.Error(), true
		}
		if r.FormValue("confirm") != "yes" {
			return "Limpeza cancelada: confirmação ausente.", true
		}
		var filter interfaces.HistoryFilter
		var label string
		switch r.FormValue("scope") {
		case "completed":
			filter.Status, label = "completed", "concluídas"
		case "failed":
			filter.Status, label = "failed", "falhas"
		case "all":
			label = "todas"
		default:
			return "Escopo de limpeza inválido.", true
		}
		n, err := q.repo.Purge(r.Context(), filter)
		if err != nil {
			log.Printf("erro ao limpar histórico: %v", err)
			return "Erro ao limpar o histórico. Veja o log do servidor.", true
		}
		return fmt.Sprintf("%d tasks %s removidas do histórico.", n, label), false
	}))
	mux.HandleFunc("/settings/failed/requeue", q.action(func(w http.ResponseWriter, r *http.Request) (string, bool) {
		requeued, skipped, err := q.RequeueFailed(r.Context())
		if err != nil {
			if errors.Is(err, ErrNoHistory) {
				return err.Error(), true
			}
			log.Printf("erro ao reenfileirar falhas: %v", err)
			return "Erro ao reenfileirar as falhas. Veja o log do servidor.", true
		}
		msg := fmt.Sprintf("%d falhas reenfileiradas.", requeued)
		if skipped > 0 {
			msg += fmt.Sprintf(" %d puladas: handler não registrado.", skipped)
		}
		return msg, skipped > 0
	}))

	return mux, nil
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
			http.Error(w, "origem não permitida", http.StatusForbidden)
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
	Flash      string
	FlashError bool
	Paused     bool
	Workers    int
	Uptime     string
	HasHistory bool
	Backend    string
	Counts     interfaces.HistoryCounts
}

func (q *Queue) buildSettings(r *http.Request) (settingsView, error) {
	view := settingsView{
		Flash:      r.URL.Query().Get("msg"),
		FlashError: r.URL.Query().Get("kind") == "erro",
		Paused:     q.Paused(),
		Workers:    q.GetWorkers(),
		Uptime:     q.Uptime().Round(time.Second).String(),
		HasHistory: q.repo != nil,
	}
	if q.repo == nil {
		return view, nil
	}
	if named, ok := q.repo.(interface{ Name() string }); ok {
		view.Backend = named.Name()
	}
	counts, err := q.repo.Counts(r.Context(), time.Time{})
	if err != nil {
		return view, fmt.Errorf("contando tasks: %w", err)
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

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("02/01/2006 15:04:05.000")
}

func fmtSpan(from, to time.Time) string {
	if from.IsZero() || to.IsZero() {
		return "—"
	}
	return to.Sub(from).Round(time.Millisecond).String()
}

func toTaskDetail(t interfaces.Task) taskDetailView {
	label := statusLabels[t.Status]
	if label[0] == "" {
		label = [2]string{t.Status, "bg-gray-100 text-gray-700"}
	}
	payload, err := json.MarshalIndent(t.Payload, "", "  ")
	if err != nil {
		payload = []byte(fmt.Sprint(t.Payload))
	}
	view := taskDetailView{
		ID:          t.ID,
		Name:        t.Name,
		Status:      t.Status,
		StatusLabel: label[0],
		StatusClass: label[1],
		Priority:    t.Priority,
		Retries:     t.Retries,
		Worker:      t.WorkerID,
		CreatedAt:   fmtTime(t.CreatedAt),
		ScheduledAt: fmtTime(t.ScheduledAt),
		StartedAt:   fmtTime(t.StartedAt),
		FinishedAt:  fmtTime(t.FinishedAt),
		QueueWait:   fmtSpan(t.CreatedAt, t.StartedAt),
		Duration:    fmtSpan(t.StartedAt, t.FinishedAt),
		Error:       t.Error,
		Payload:     string(payload),
		CanRequeue:  t.Status == "failed",
	}
	for _, a := range t.Attempts {
		view.Attempts = append(view.Attempts, attemptView{
			Number:   a.Number,
			Started:  fmtTime(a.StartedAt),
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
		log.Printf("erro ao carregar task: %v", err)
		http.Error(w, "erro ao consultar o histórico", http.StatusInternalServerError)
		return
	}

	view := toTaskDetail(t)
	view.Flash = r.URL.Query().Get("msg")
	view.FlashError = r.URL.Query().Get("kind") == "erro"

	tmpl, err := q.uiTemplates()
	if err != nil {
		http.Error(w, "erro ao carregar templates", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "task-detail.html", view); err != nil {
		log.Printf("erro ao renderizar task-detail: %v", err)
	}
}

// taskLog devolve um arquivo de texto com os dados da execução, para anexar
// a um chamado ou comparar execuções.
func (q *Queue) taskLog(w http.ResponseWriter, r *http.Request) {
	if q.repo == nil {
		http.Error(w, ErrNoHistory.Error(), http.StatusNotFound)
		return
	}
	id := r.PathValue("id")
	t, err := q.repo.Get(r.Context(), id)
	if errors.Is(err, interfaces.ErrTaskNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("erro ao gerar log: %v", err)
		http.Error(w, "erro ao consultar o histórico", http.StatusInternalServerError)
		return
	}

	v := toTaskDetail(t)
	var b strings.Builder
	fmt.Fprintf(&b, "Task:        %s\n", v.ID)
	fmt.Fprintf(&b, "Nome:        %s\n", v.Name)
	fmt.Fprintf(&b, "Status:      %s\n", v.Status)
	fmt.Fprintf(&b, "Prioridade:  %d\n", v.Priority)
	fmt.Fprintf(&b, "Criada em:   %s\n", v.CreatedAt)
	fmt.Fprintf(&b, "Agendada:    %s\n", v.ScheduledAt)
	fmt.Fprintf(&b, "Iniciada em: %s\n", v.StartedAt)
	fmt.Fprintf(&b, "Finalizada:  %s\n", v.FinishedAt)
	fmt.Fprintf(&b, "Espera:      %s\n", v.QueueWait)
	fmt.Fprintf(&b, "Duração:     %s\n", v.Duration)
	fmt.Fprintf(&b, "Worker:      %s\n", v.Worker)
	fmt.Fprintf(&b, "\nPayload:\n%s\n", v.Payload)
	fmt.Fprintf(&b, "\nTentativas (%d):\n", len(v.Attempts))
	for _, a := range v.Attempts {
		result := "sucesso"
		if a.Failed {
			result = "ERRO: " + a.Error
		}
		fmt.Fprintf(&b, "  #%d  início=%s  duração=%s  worker=%s  %s\n", a.Number, a.Started, a.Duration, a.Worker, result)
	}
	if v.Error != "" {
		fmt.Fprintf(&b, "\nÚltimo erro: %s\n", v.Error)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.log"`, id))
	io.WriteString(w, b.String())
}

// ServeUI sobe o painel em addr e para quando ctx é cancelado.
func (q *Queue) ServeUI(addr string, ctx context.Context) {
	handler, err := q.uiHandler()
	if err != nil {
		log.Fatalf("erro ao iniciar UI: %v", err)
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: middlewares.HTTPLoggingMiddleware(log.New(os.Stdout, "", log.LstdFlags))(handler),
	}

	go func() {
		<-ctx.Done()
		log.Println("🛑 Encerrando servidor UI...")
		srv.Shutdown(context.Background())
	}()

	go func() {
		log.Printf("🌐 Servindo UI em http://%s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro ao iniciar UI: %v", err)
		}
	}()
}

// streamEvents envia uma mensagem SSE por mudança de estado. O payload é só
// o ID e o status: o navegador recarrega a tabela e os cards a partir do
// histórico, então não precisa montar HTML aqui.
func (q *Queue) streamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming não suportado", http.StatusInternalServerError)
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
