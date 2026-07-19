package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func makeBundle(t *testing.T) string {
	t.Helper()
	b := t.TempDir()
	write(t, filepath.Join(b, "CLAUDE.md"), "cfg\n")
	write(t, filepath.Join(b, ".claude/rules/x.md"), "rule\n")
	return b
}

// resultsWithBundle writes a results dir whose run records app and bundle.
func resultsWithBundle(t *testing.T, root, app, artifact, bundle string) string {
	t.Helper()
	dir := filepath.Join(root, "res")
	write(t, filepath.Join(dir, "bench.json"), `{"app":`+strconv.Quote(app)+`}`)
	meta := `{"config":"c"}`
	if bundle != "" {
		meta = `{"config":"c","bundle":` + strconv.Quote(bundle) + `}`
	}
	write(t, filepath.Join(dir, artifact, "meta.json"), meta)
	return "res"
}

func TestConfigFilesLists(t *testing.T) {
	srv, root := newTestServer(t)
	resultsWithBundle(t, root, t.TempDir(), "cfg", makeBundle(t))

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/config-files?dir=res&artifact=cfg", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body:\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "CLAUDE.md") || !strings.Contains(body, ".claude/rules/x.md") {
		t.Errorf("config-files should list bundle files; got:\n%s", body)
	}
}

func TestConfigFilesErrors(t *testing.T) {
	tests := []struct {
		name   string
		dir    string
		bundle string // "" → meta without bundle; "missing" → nonexistent path
		want   int
	}{
		{name: "bad dir", dir: "../../etc", want: http.StatusBadRequest},
		{name: "bundle unknown", dir: "res", bundle: "", want: http.StatusBadRequest},
		{name: "bundle missing", dir: "res", bundle: "missing", want: http.StatusBadRequest},
		{name: "bundle is a file", dir: "res", bundle: "file", want: http.StatusBadRequest},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			srv, root := newTestServer(t)
			bundle := tt.bundle
			switch tt.bundle {
			case "missing":
				bundle = filepath.Join(root, "nope-bundle")
			case "file":
				bundle = filepath.Join(root, "bundle-file")
				write(t, bundle, "not a dir")
			}
			if tt.dir == "res" {
				resultsWithBundle(t, root, t.TempDir(), "cfg", bundle)
			}
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/config-files?dir="+url.QueryEscape(tt.dir)+"&artifact=cfg", nil))
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d; body:\n%s", rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}

func TestConfigFilesRejectsBadArtifact(t *testing.T) {
	srv, root := newTestServer(t)
	resultsWithBundle(t, root, t.TempDir(), "cfg", makeBundle(t))

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/config-files?dir=res&artifact="+url.QueryEscape("../../x"), nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func postExport(dir, artifact string, files ...string) *http.Request {
	form := url.Values{"dir": {dir}, "artifact": {artifact}}
	for _, f := range files {
		form.Add("files", f)
	}
	req := httptest.NewRequest(http.MethodPost, "/export-config", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestExportConfigCopiesSelectedFiles(t *testing.T) {
	srv, root := newTestServer(t)
	app := t.TempDir()
	resultsWithBundle(t, root, app, "cfg", makeBundle(t))

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, postExport("res", "cfg", "CLAUDE.md", ".claude/rules/x.md"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body:\n%s", rec.Code, rec.Body.String())
	}
	for _, rel := range []string{"CLAUDE.md", ".claude/rules/x.md"} {
		if _, err := os.Stat(filepath.Join(app, rel)); err != nil {
			t.Errorf("expected %s exported into app: %v", rel, err)
		}
	}
	if !strings.Contains(rec.Body.String(), "2 fichiers de conf exportés") {
		t.Errorf("body should confirm 2 files; got:\n%s", rec.Body.String())
	}
}

func TestExportConfigSingleFileMessage(t *testing.T) {
	srv, root := newTestServer(t)
	resultsWithBundle(t, root, t.TempDir(), "cfg", makeBundle(t))

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, postExport("res", "cfg", "CLAUDE.md"))

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "1 fichier de conf exporté") {
		t.Fatalf("status = %d, body:\n%s", rec.Code, rec.Body.String())
	}
}

func TestExportConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		app     string // "real" → a temp dir; "file" → a regular file; "" → empty
		bundle  bool   // record a real bundle
		files   []string
		want    int
		wantMsg string
	}{
		{name: "bad dir", dir: "../../etc", want: http.StatusBadRequest, wantMsg: "racine"},
		{name: "bundle unknown", dir: "res", app: "real", bundle: false, files: []string{"CLAUDE.md"}, want: http.StatusBadRequest, wantMsg: "bundle inconnu"},
		{name: "app missing", dir: "res", app: "", bundle: true, files: []string{"CLAUDE.md"}, want: http.StatusBadRequest, wantMsg: "app introuvable"},
		{name: "no files", dir: "res", app: "real", bundle: true, want: http.StatusBadRequest, wantMsg: "aucun fichier"},
		{name: "invalid path", dir: "res", app: "real", bundle: true, files: []string{"../evil"}, want: http.StatusUnprocessableEntity, wantMsg: "invalide"},
		{name: "missing file", dir: "res", app: "real", bundle: true, files: []string{"ghost.md"}, want: http.StatusUnprocessableEntity, wantMsg: "export interrompu"},
		{name: "app is a file", dir: "res", app: "file", bundle: true, files: []string{"CLAUDE.md"}, want: http.StatusUnprocessableEntity, wantMsg: "export interrompu"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			srv, root := newTestServer(t)
			var app string
			switch tt.app {
			case "real":
				app = t.TempDir()
			case "file":
				app = filepath.Join(root, "appfile")
				write(t, app, "x")
			}
			bundle := ""
			if tt.bundle {
				bundle = makeBundle(t)
			}
			if tt.dir == "res" {
				resultsWithBundle(t, root, app, "cfg", bundle)
			}

			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, postExport(tt.dir, "cfg", tt.files...))

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d; body:\n%s", rec.Code, tt.want, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.wantMsg) {
				t.Errorf("body should mention %q; got:\n%s", tt.wantMsg, rec.Body.String())
			}
		})
	}
}
