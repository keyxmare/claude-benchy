// Package server exposes the benchy web dashboard: a form to configure and
// launch a benchmark, live progress for a running benchmark, and the history of
// every benchmark produced under a root directory. It reuses the same pipeline
// as the CLI (runner.Run, runner.Reload, report.WriteHTML) and adds no runtime
// dependency beyond the standard library.
package server

import (
	"bufio"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/docker"
	"github.com/keyxmare/claude-benchy/internal/report"
	"github.com/keyxmare/claude-benchy/internal/runner"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

var tmpl = template.Must(template.ParseFS(templatesFS, "templates/*.html"))

// Server serves the dashboard over HTTP.
type Server struct {
	root   string        // scanned for past benchmarks; base for the form's relative paths
	image  string        // default sandbox image
	docker docker.Runner // sandbox runner injected by the caller
	jobs   *jobManager
}

// New returns a Server listing and running benchmarks under root, using image
// as the default sandbox image and d to run containers.
func New(root, image string, d docker.Runner) (*Server, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Server{root: abs, image: image, docker: d, jobs: newJobManager()}, nil
}

// Handler builds the HTTP routing for the dashboard.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleDashboard)
	mux.HandleFunc("GET /new", s.handleNew)
	mux.HandleFunc("POST /run", s.handleRun)
	mux.HandleFunc("POST /apply", s.handleApply)
	mux.HandleFunc("GET /config-files", s.handleConfigFiles)
	mux.HandleFunc("POST /export-config", s.handleExportConfig)
	mux.HandleFunc("GET /runs/{id}", s.handleRunPage)
	mux.HandleFunc("GET /runs/{id}/events", s.handleEvents)
	mux.HandleFunc("POST /runs/{id}/stop", s.handleStop)
	mux.HandleFunc("GET /report", s.handleReport)
	mux.HandleFunc("GET /transcript", s.handleTranscript)
	mux.HandleFunc("GET /browse", s.handleBrowse)
	mux.HandleFunc("GET /file", s.handleFile)
	mux.Handle("GET /static/", http.FileServerFS(staticFS))
	return mux
}

// dashboardData is the model for the dashboard page.
type dashboardData struct {
	History []historyEntry
	Form    formValues
	Error   string
	Models  []string
	Root    string
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.renderDashboard(w, defaultForm(), "", http.StatusOK)
}

// handleNew renders the dashboard with the form pre-filled: from an existing
// bench.yaml file (?import=) or from a past run's directory (?from=), so a
// configuration can be reviewed, adapted and re-launched.
func (s *Server) handleNew(w http.ResponseWriter, r *http.Request) {
	if imp := r.URL.Query().Get("import"); imp != "" {
		path, err := s.resolveExistingFile(imp)
		if err != nil {
			s.renderDashboard(w, defaultForm(), err.Error(), http.StatusBadRequest)
			return
		}
		fv, err := formFromFile(path)
		if err != nil {
			s.renderDashboard(w, defaultForm(), err.Error(), http.StatusBadRequest)
			return
		}
		s.renderDashboard(w, fv, "", http.StatusOK)
		return
	}

	from := r.URL.Query().Get("from")
	if from == "" {
		s.renderDashboard(w, defaultForm(), "", http.StatusOK)
		return
	}
	dir, err := s.safeDir(from)
	if err != nil {
		s.renderDashboard(w, defaultForm(), err.Error(), http.StatusBadRequest)
		return
	}
	s.renderDashboard(w, formFromDir(s.root, dir), "", http.StatusOK)
}

// resolveExistingFile turns a picker path (relative to the root, or absolute)
// into an existing regular file. Like the browse/file endpoints it is not
// confined to the root: the server is localhost-only and already reads the
// arbitrary files the user selects.
func (s *Server) resolveExistingFile(p string) (string, error) {
	if strings.TrimSpace(p) == "" {
		return "", errors.New("chemin manquant")
	}
	abs := filepath.Clean(p)
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(s.root, abs)
	}
	if info, err := os.Stat(abs); err != nil || info.IsDir() {
		return "", errors.New("fichier introuvable")
	}
	return abs, nil
}

func (s *Server) renderDashboard(w http.ResponseWriter, form formValues, errMsg string, code int) {
	data := dashboardData{
		History: scanHistory(s.root),
		Form:    form,
		Error:   errMsg,
		Models:  []string{"sonnet", "opus", "haiku"},
		Root:    s.root,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := tmpl.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		fmt.Fprintf(os.Stderr, "render dashboard: %v\n", err)
	}
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	form := readForm(r)
	sp, err := form.spec()
	if err != nil {
		s.renderDashboard(w, form, err.Error(), http.StatusBadRequest)
		return
	}
	// Marshal the bench before Build resolves paths to absolute (and inlines
	// promptFile contents into each config), so the persisted bench.yaml stays
	// readable and re-runnable.
	raw, err := yaml.Marshal(benchDocFrom(sp))
	if err != nil {
		s.renderDashboard(w, form, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := sp.Build(s.root); err != nil {
		s.renderDashboard(w, form, err.Error(), http.StatusBadRequest)
		return
	}
	j, err := s.jobs.start(&sp, string(raw), chooseImage(s.image, sp.Sandbox.Image), s.docker)
	if err != nil {
		s.renderDashboard(w, form, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/runs/"+j.id, http.StatusSeeOther)
}

// chooseImage mirrors the CLI's resolution: the bench's own sandbox.image wins
// over the server's default image, so a bench declaring a toolchain image
// (e.g. one bundling Go for its checks) runs in it.
func chooseImage(serverDefault, benchImage string) string {
	if benchImage != "" {
		return benchImage
	}
	return serverDefault
}

// runPageData is the model for a run's live-progress page.
type runPageData struct {
	ID        string
	Status    string
	Logs      []string
	ReportDir string
}

func (s *Server) handleRunPage(w http.ResponseWriter, r *http.Request) {
	j, ok := s.jobs.get(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	events, status, _, _ := j.snapshot(0)
	var logs []string
	for _, e := range events {
		if e.Kind == "log" {
			logs = append(logs, e.Line)
		}
	}
	rel, _ := filepath.Rel(s.root, j.outputRoot)
	data := runPageData{ID: j.id, Status: string(status), Logs: logs, ReportDir: rel}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "run.html", data); err != nil {
		fmt.Fprintf(os.Stderr, "render run page: %v\n", err)
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	j, ok := s.jobs.get(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming non supporté", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx := r.Context()
	i := 0
	for {
		events, status, errMsg, changed := j.snapshot(i)
		for _, ev := range events {
			if ev.Kind == "agent" {
				if payload, err := json.Marshal(ev); err == nil {
					writeSSE(w, "agent", string(payload))
				}
			} else {
				writeSSE(w, "log", ev.Line)
			}
			i++
		}
		if status != statusRunning {
			payload := string(status)
			if errMsg != "" {
				payload += ": " + errMsg
			}
			writeSSE(w, "done", payload)
			flusher.Flush()
			return
		}
		flusher.Flush()
		select {
		case <-ctx.Done():
			return
		case <-changed:
		}
	}
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	j, ok := s.jobs.get(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	j.cancel()
	http.Redirect(w, r, "/runs/"+j.id, http.StatusSeeOther)
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	dir, err := s.safeDir(r.URL.Query().Get("dir"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	prompt, app := runner.BenchInfo(dir)
	rep, err := runner.Reload(dir, app, prompt, time.Now().Format(generatedAtLayout))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rep.HomeURL = "/"
	rep.ReuseURL = "/new?from=" + url.QueryEscape(r.URL.Query().Get("dir"))
	if rel, err := filepath.Rel(s.root, dir); err == nil {
		rep.TranscriptBase = rel
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := report.WriteHTML(w, rep); err != nil {
		fmt.Fprintf(os.Stderr, "render report: %v\n", err)
	}
}

// transcriptData is the model for a single agent's replayed transcript page.
type transcriptData struct {
	Agent string
	Lines []string
}

// handleTranscript replays one agent's transcript.jsonl as the same readable
// feed shown live during the run, so it stays consultable after completion and
// across server restarts.
func (s *Server) handleTranscript(w http.ResponseWriter, r *http.Request) {
	dir, err := s.withinRoot(r.URL.Query().Get("dir"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f, err := os.Open(filepath.Join(dir, "transcript.jsonl"))
	if err != nil {
		http.Error(w, "transcript introuvable", http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		lines = append(lines, claude.Render(scanner.Bytes())...)
	}
	rel, _ := filepath.Rel(s.root, dir)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "transcript.html", transcriptData{Agent: rel, Lines: lines}); err != nil {
		fmt.Fprintf(os.Stderr, "render transcript: %v\n", err)
	}
}

// withinRoot resolves rel against the root and rejects anything escaping it.
func (s *Server) withinRoot(rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", errors.New("paramètre dir manquant")
	}
	abs := filepath.Clean(rel)
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(s.root, abs)
	}
	rc, err := filepath.Rel(s.root, abs)
	if err != nil || rc == ".." || strings.HasPrefix(rc, ".."+string(filepath.Separator)) {
		return "", errors.New("chemin hors de la racine")
	}
	return abs, nil
}

// safeDir resolves a history entry's dir parameter and requires it to look like
// a benchmark output directory.
func (s *Server) safeDir(rel string) (string, error) {
	abs, err := s.withinRoot(rel)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(abs, "bench.json")); err != nil {
		return "", errors.New("dossier de bench introuvable")
	}
	return abs, nil
}

// writeSSE emits one Server-Sent Event, splitting multi-line data across the
// `data:` fields the protocol requires.
func writeSSE(w io.Writer, event, data string) {
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(data, "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	_, _ = fmt.Fprint(w, "\n")
}
