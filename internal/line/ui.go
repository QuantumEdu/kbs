package line

import (
	"context"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
)

const defaultListen = "127.0.0.1:7340"

type UI struct {
	engine *Engine
	mux    *http.ServeMux
}

func NewUI(engine *Engine) *UI {
	ui := &UI{engine: engine, mux: http.NewServeMux()}
	ui.mux.HandleFunc("GET /{$}", ui.home)
	ui.mux.HandleFunc("GET /jobs/{id}", ui.show)
	ui.mux.HandleFunc("POST /jobs/{id}/continue", ui.continueJob)
	ui.mux.HandleFunc("POST /jobs/{id}/advance", ui.advanceJob)
	ui.mux.HandleFunc("POST /jobs/new", ui.createJob)
	ui.mux.HandleFunc("POST /job/new", ui.createJob)
	return ui
}

func (ui *UI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ui.mux.ServeHTTP(w, r)
}

func (ui *UI) home(w http.ResponseWriter, r *http.Request) {
	jobs, err := ui.engine.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if jobs == nil {
		jobs = []Job{}
	}
	render(w, pageList, map[string]any{"Jobs": jobs})
}

func (ui *UI) show(w http.ResponseWriter, r *http.Request) {
	job, events, err := ui.engine.Status(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	render(w, pageJob, map[string]any{"Job": job, "Events": events})
}

func (ui *UI) continueJob(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if _, err := ui.engine.Continue(r.Context(), id, r.FormValue("answer")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/jobs/"+id, http.StatusSeeOther)
}

func (ui *UI) advanceJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := ui.engine.Advance(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/jobs/"+id, http.StatusSeeOther)
}

func (ui *UI) createJob(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	issueURL := strings.TrimSpace(r.FormValue("issue_url"))
	if issueURL == "" {
		issueURL = strings.TrimSpace(r.FormValue("issue"))
	}
	repoPath := strings.TrimSpace(r.FormValue("repo_path"))
	if repoPath == "" {
		repoPath = strings.TrimSpace(r.FormValue("repo"))
	}
	if issueURL == "" || repoPath == "" {
		http.Error(w, "issue_url and repo_path are required", http.StatusBadRequest)
		return
	}
	job, err := ui.engine.RunPlan(r.Context(), issueURL, repoPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/jobs/"+job.ID, http.StatusSeeOther)
}

func render(w http.ResponseWriter, tmpl *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func ListenAndServe(ctx context.Context, addr string, handler http.Handler) error {
	if strings.TrimSpace(addr) == "" {
		addr = defaultListen
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("line serve: refuses non-loopback address %q (must be 127.0.0.1)", addr)
	}
	server := &http.Server{Addr: addr, Handler: handler}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	err = server.ListenAndServe()
	if err == http.ErrServerClosed {
		return ctx.Err()
	}
	return err
}

var (
	pageList = template.Must(template.New("list").Parse(uiLayout + uiList))
	pageJob  = template.Must(template.New("job").Parse(uiLayout + uiJob))
)

const uiLayout = `<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>line</title>
<style>
body{font:16px/1.4 system-ui,sans-serif;margin:0;background:#111;color:#eee}
main{max-width:52rem;margin:0 auto;padding:1.5rem}
a{color:#8ec8ff}
table{width:100%;border-collapse:collapse}
td,th{text-align:left;padding:.4rem .5rem;border-bottom:1px solid #333}
.badge{font:12px monospace;padding:.1rem .4rem;border:1px solid #555;border-radius:4px}
form{margin:1rem 0}
input,button,textarea{font:inherit;padding:.4rem}
textarea{width:100%;min-height:4rem}
.muted{color:#aaa}
</style>
<main>
<p><a href="/">line</a> <span class="muted">local factory · ntfy optional</span></p>
`

const uiList = `
<h1>Jobs</h1>
<section style="margin-bottom: 2rem; padding: 1rem; border: 1px solid #333; border-radius: 6px;">
<h2>Start New Job</h2>
<form method="post" action="/jobs/new">
<p><label>Issue URL<br><input type="url" name="issue_url" required placeholder="https://github.com/owner/repo/issues/1" style="width:100%"></label></p>
<p><label>Repo Path<br><input type="text" name="repo_path" required placeholder="/absolute/path/to/repo" style="width:100%"></label></p>
<p><button type="submit">Start Job</button></p>
</form>
</section>
{{if .Jobs}}
<table>
<tr><th>id</th><th>phase</th><th>status</th><th>issue</th></tr>
{{range .Jobs}}
<tr>
<td><a href="/jobs/{{.ID}}">{{.ID}}</a></td>
<td class="badge">{{.Phase}}</td>
<td class="badge">{{.Status}}</td>
<td><a href="{{.IssueURL}}">{{.IssueURL}}</a></td>
</tr>
{{end}}
</table>
{{else}}
<p class="muted">No jobs yet. Use <code>line run</code> or the form above.</p>
{{end}}
</main>`

const uiJob = `
<h1>Job {{.Job.ID}}</h1>
<p>phase <span class="badge">{{.Job.Phase}}</span> status <span class="badge">{{.Job.Status}}</span></p>
<p><a href="{{.Job.IssueURL}}">{{.Job.IssueURL}}</a></p>
{{if .Job.Summary}}<p>{{.Job.Summary}}</p>{{end}}
{{if .Job.Question}}<p><strong>question:</strong> {{.Job.Question}}</p>{{end}}
{{if eq .Job.Status "needs_human"}}
<form method="post" action="/jobs/{{.Job.ID}}/continue">
<label>answer<br><textarea name="answer" required></textarea></label>
<p><button type="submit">continue</button></p>
</form>
{{end}}
{{if eq .Job.Status "awaiting_next"}}
<form method="post" action="/jobs/{{.Job.ID}}/advance">
<button type="submit">advance {{.Job.Phase}}</button>
</form>
{{end}}
<h2>Events</h2>
<ul>
{{range .Events}}<li><span class="badge">{{.Phase}}</span> {{.Kind}} {{.Detail}}</li>{{end}}
</ul>
</main>`
