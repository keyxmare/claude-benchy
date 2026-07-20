package server_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keyxmare/claude-benchy/internal/docker"
	"github.com/keyxmare/claude-benchy/internal/server"
)

// fakeDocker stands in for the sandbox: a Claude run mutates the workspace and
// emits a stream-json result; a check run (entrypoint override) passes. This
// exercises the server end to end without Docker or the network.
type fakeDocker struct{}

func (fakeDocker) Build(context.Context, string, string, string, map[string]string) error {
	return nil
}

func (fakeDocker) Run(_ context.Context, spec docker.RunSpec, stdout, _ io.Writer) error {
	if spec.Entrypoint != "" {
		return nil
	}
	_ = os.WriteFile(filepath.Join(spec.WorkDir, "added.txt"), []byte("edited\n"), 0o644)
	_, _ = fmt.Fprintln(stdout, `{"type":"result","subtype":"success","is_error":false,"num_turns":1,"total_cost_usd":0.01,"result":"ok"}`)
	return nil
}

// benchRoot lays out a temp dir holding an app and a config bundle, ready for a
// benchmark, and returns its path.
func benchRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	app := filepath.Join(root, "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "main.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "config-baseline")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "CLAUDE.md"), []byte("cfg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// newTestServer returns a server rooted at a temp dir holding an app and a
// config bundle, ready to run a benchmark through the fake sandbox.
func newTestServer(t *testing.T) (*server.Server, string) {
	t.Helper()
	root := benchRoot(t)
	srv, err := server.New(root, "img", fakeDocker{})
	if err != nil {
		t.Fatal(err)
	}
	return srv, root
}

// serverWith returns a server over a fresh bench root driven by the given
// sandbox runner, for the lifecycle branches the default fake cannot reach.
func serverWith(t *testing.T, d docker.Runner) (*server.Server, string) {
	t.Helper()
	root := benchRoot(t)
	srv, err := server.New(root, "img", d)
	if err != nil {
		t.Fatal(err)
	}
	return srv, root
}

// latestRunDir returns the single run directory produced under results/,
// relative to root — the observable artifact of a completed benchmark, found
// without reaching into the server's unexported history scan.
func latestRunDir(t *testing.T, root string) string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(root, "results"))
	if err != nil {
		t.Fatalf("read results: %v", err)
	}
	if len(ents) != 1 {
		t.Fatalf("results has %d entries, want 1", len(ents))
	}
	return filepath.Join("results", ents[0].Name())
}

func runForm() url.Values {
	return url.Values{
		"prompt":       {"do the thing"},
		"app":          {"./app"},
		"model":        {"sonnet"},
		"runs":         {"1"},
		"concurrency":  {"1"},
		"output":       {"results"},
		"configName":   {"baseline"},
		"configBundle": {"./config-baseline"},
		"configModel":  {""},
		"configPrompt": {""},
	}
}

func TestDashboardRenders(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Nouveau bench") {
		t.Error("dashboard should render the new-bench form")
	}
}

func TestRunLaunchesJobAndStreamsToCompletion(t *testing.T) {
	srv, root := newTestServer(t)
	h := srv.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(runForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body: %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if loc != "/runs/1" {
		t.Fatalf("redirect = %q, want /runs/1", loc)
	}

	// The events stream blocks until the (fast) fake run finishes.
	evRec := httptest.NewRecorder()
	h.ServeHTTP(evRec, httptest.NewRequest(http.MethodGet, "/runs/1/events", nil))
	stream := evRec.Body.String()
	if !strings.Contains(stream, "event: log") {
		t.Errorf("stream should carry progress logs, got:\n%s", stream)
	}
	if !strings.Contains(stream, "event: agent") {
		t.Errorf("stream should carry per-agent events, got:\n%s", stream)
	}
	if !strings.Contains(stream, "event: done\ndata: done") {
		t.Errorf("stream should end with a done event, got:\n%s", stream)
	}

	// The completed run is listed on the dashboard, summarised by its prompt.
	dashRec := httptest.NewRecorder()
	h.ServeHTTP(dashRec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(dashRec.Body.String(), "do the thing") {
		t.Errorf("dashboard should list the run by its prompt, got:\n%s", dashRec.Body.String())
	}

	dir := latestRunDir(t, root)

	// The freshly produced run renders through the report route.
	repRec := httptest.NewRecorder()
	h.ServeHTTP(repRec, httptest.NewRequest(http.MethodGet, "/report?dir="+url.QueryEscape(dir), nil))
	if repRec.Code != http.StatusOK {
		t.Fatalf("report status = %d, want 200; body: %s", repRec.Code, repRec.Body.String())
	}
	if !strings.Contains(repRec.Body.String(), "baseline") {
		t.Error("report should mention the config")
	}
	if !strings.Contains(repRec.Body.String(), "/transcript?dir=") {
		t.Error("served report should link each run to its transcript")
	}

	// The per-agent transcript replays the same readable feed from disk.
	trRec := httptest.NewRecorder()
	h.ServeHTTP(trRec, httptest.NewRequest(http.MethodGet, "/transcript?dir="+url.QueryEscape(dir+"/baseline"), nil))
	if trRec.Code != http.StatusOK {
		t.Fatalf("transcript status = %d, want 200; body: %s", trRec.Code, trRec.Body.String())
	}
	if !strings.Contains(trRec.Body.String(), "✓ terminé") {
		t.Errorf("transcript should replay the rendered feed, got:\n%s", trRec.Body.String())
	}
}

func TestTranscriptRejectsPathTraversal(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/transcript?dir=../../etc", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an out-of-root dir", rec.Code)
	}
}

func TestNewPrefillsFormFromRunConfig(t *testing.T) {
	srv, root := newTestServer(t)
	h := srv.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(runForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)

	// Wait for completion (and thus the persisted bench.yaml) via the stream.
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/runs/1/events", nil))

	dir := latestRunDir(t, root)

	newRec := httptest.NewRecorder()
	h.ServeHTTP(newRec, httptest.NewRequest(http.MethodGet, "/new?from="+url.QueryEscape(dir), nil))
	if newRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", newRec.Code)
	}
	body := newRec.Body.String()
	if !strings.Contains(body, "do the thing") {
		t.Error("reused form should carry the prompt")
	}
	if !strings.Contains(body, `value="baseline"`) || !strings.Contains(body, `value="./config-baseline"`) {
		t.Errorf("reused form should carry the config name and bundle; body:\n%s", body)
	}
}

func TestRunRerendersFormOnValidationError(t *testing.T) {
	srv, _ := newTestServer(t)
	form := runForm()
	form.Set("app", "") // app is required

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "app is required") {
		t.Errorf("expected validation error surfaced in the form, got:\n%s", body)
	}
	if !strings.Contains(body, `value="do the thing"`) && !strings.Contains(body, "do the thing") {
		t.Error("expected the submitted values to be repopulated")
	}
}

func TestBrowseListsDirectories(t *testing.T) {
	srv, root := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/browse?mode=dir&path="+url.QueryEscape(root), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"app"`) || !strings.Contains(body, `"config-baseline"`) {
		t.Errorf("browse should list the app and bundle directories, got:\n%s", body)
	}
	if strings.Contains(body, `"main.txt"`) {
		t.Error("dir mode should not list regular files")
	}
}

func TestFileServesTextAndRejectsDir(t *testing.T) {
	srv, root := newTestServer(t)
	h := srv.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/file?path="+url.QueryEscape(filepath.Join(root, "app", "main.txt")), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "hello\n" {
		t.Errorf("content = %q, want %q", rec.Body.String(), "hello\n")
	}

	dirRec := httptest.NewRecorder()
	h.ServeHTTP(dirRec, httptest.NewRequest(http.MethodGet, "/file?path="+url.QueryEscape(filepath.Join(root, "app")), nil))
	if dirRec.Code != http.StatusNotFound {
		t.Errorf("reading a directory: status = %d, want 404", dirRec.Code)
	}
}

// TestReuseRecoversBundleFromAncestorBench covers a legacy CLI run whose own
// directory carries no bench.yaml: the config (bundle included) is recovered
// from the source bench.yaml sitting above the results directory.
func TestReuseRecoversBundleFromAncestorBench(t *testing.T) {
	srv, root := newTestServer(t)

	if err := os.WriteFile(filepath.Join(root, "bench.yaml"), []byte(
		"prompt: do it\napp: ./app\nconfigs:\n  - name: baseline\n    bundle: ./config-baseline\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, "results", "20260101-000000")
	if err := os.MkdirAll(filepath.Join(runDir, "baseline"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "bench.json"), []byte(`{"prompt":"do it","app":"/abs/app"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "baseline", "diff.patch"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/new?from="+url.QueryEscape("results/20260101-000000"), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `value="./config-baseline"`) {
		t.Errorf("bundle should be recovered from the ancestor bench.yaml; body:\n%s", rec.Body.String())
	}
}

func TestReportRejectsPathTraversal(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/report?dir=../../etc", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an out-of-root dir", rec.Code)
	}
}

func TestDeleteHistoryRemovesRunAndRejectsBadDir(t *testing.T) {
	srv, root := newTestServer(t)
	runDir := filepath.Join(root, "results", "20260101-000000")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "bench.json"), []byte(`{"prompt":"do it"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("removes a genuine run directory", func(t *testing.T) {
		form := strings.NewReader("dir=" + url.QueryEscape("results/20260101-000000"))
		req := httptest.NewRequest(http.MethodPost, "/history/delete", form)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want 303", rec.Code)
		}
		if _, err := os.Stat(runDir); !os.IsNotExist(err) {
			t.Errorf("run directory should be removed, stat err = %v", err)
		}
	})

	t.Run("rejects a dir without a bench.json", func(t *testing.T) {
		form := strings.NewReader("dir=" + url.QueryEscape("results"))
		req := httptest.NewRequest(http.MethodPost, "/history/delete", form)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 for a non-bench dir", rec.Code)
		}
	})
}
