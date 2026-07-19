package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.email=t@example.com", "-c", "user.name=test", "-c", "commit.gpgsign=false"}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// appRepoAndPatch creates a git repo and returns a patch that, once applied,
// appends a line to a tracked file. The change is reverted so the patch is
// pending.
func appRepoAndPatch(t *testing.T) (app, patch string) {
	t.Helper()
	app = t.TempDir()
	gitRun(t, app, "init")
	write(t, filepath.Join(app, "tracked.txt"), "line1\n")
	gitRun(t, app, "add", ".")
	gitRun(t, app, "commit", "-m", "base")
	write(t, filepath.Join(app, "tracked.txt"), "line1\nline2\n")
	out, err := exec.Command("git", "-C", app, "diff").CombinedOutput()
	if err != nil {
		t.Fatalf("git diff: %v\n%s", err, out)
	}
	gitRun(t, app, "checkout", "--", ".")
	return app, string(out)
}

// results writes a benchmark output directory under root with a bench.json and
// one artifact holding a diff.patch, returning the dir param (relative to root).
func results(t *testing.T, root, app, artifact, patch string) string {
	t.Helper()
	dir := filepath.Join(root, "res")
	write(t, filepath.Join(dir, "bench.json"), `{"app":`+strconv.Quote(app)+`}`)
	write(t, filepath.Join(dir, artifact, "diff.patch"), patch)
	return "res"
}

func postApply(dir, artifact string) *http.Request {
	form := url.Values{"dir": {dir}, "artifact": {artifact}}
	req := httptest.NewRequest(http.MethodPost, "/apply", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestApplyAppliesPatchToProject(t *testing.T) {
	srv, root := newTestServer(t)
	app, patch := appRepoAndPatch(t)
	dir := results(t, root, app, "cfg", patch)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, postApply(dir, "cfg"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body:\n%s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(app, "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "line1\nline2\n" {
		t.Errorf("tracked.txt = %q, want the patch applied", got)
	}
	if !strings.Contains(rec.Body.String(), "appliquées") {
		t.Errorf("page should confirm success; body:\n%s", rec.Body.String())
	}
}

func TestApplyErrors(t *testing.T) {
	tests := []struct {
		name     string
		dir      string // dir param; "res" means the standard results dir
		artifact string
		app      string // app recorded in bench.json ("repo" = a real repo)
		patch    string // patch content ("valid" = the pending patch)
		wantCode int
		wantMsg  string
	}{
		{name: "dir outside root", dir: "../../etc", artifact: "cfg", wantCode: http.StatusBadRequest, wantMsg: "racine"},
		{name: "artifact missing", dir: "res", artifact: "", app: "repo", patch: "valid", wantCode: http.StatusBadRequest, wantMsg: "artefact manquant"},
		{name: "artifact escapes results", dir: "res", artifact: "../../elsewhere", app: "repo", patch: "valid", wantCode: http.StatusBadRequest, wantMsg: "hors du dossier"},
		{name: "missing patch", dir: "res", artifact: "empty", app: "repo", patch: "valid", wantCode: http.StatusBadRequest, wantMsg: "diff introuvable"},
		{name: "app missing", dir: "res", artifact: "cfg", app: "", patch: "valid", wantCode: http.StatusBadRequest, wantMsg: "app introuvable"},
		{name: "git apply fails", dir: "res", artifact: "cfg", app: "repo", patch: "ceci n'est pas un patch\n", wantCode: http.StatusUnprocessableEntity, wantMsg: "git apply"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			srv, root := newTestServer(t)
			app, valid := appRepoAndPatch(t)
			appPath := ""
			if tt.app == "repo" {
				appPath = app
			}
			patch := tt.patch
			if patch == "valid" {
				patch = valid
			}
			// Always lay down the standard results dir; individual cases point
			// their artifact param elsewhere to trigger the error.
			if tt.dir == "res" {
				results(t, root, appPath, "cfg", patch)
			}

			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, postApply(tt.dir, tt.artifact))

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d; body:\n%s", rec.Code, tt.wantCode, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.wantMsg) {
				t.Errorf("body should mention %q; got:\n%s", tt.wantMsg, rec.Body.String())
			}
		})
	}
}
