package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRootBench drops a bench.yaml under the server root and returns its path
// relative to the root, as the picker would supply it.
func writeRootBench(t *testing.T, root, body string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "bench.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return "./bench.yaml"
}

func TestNewImportsBenchFileWithAbsolutePaths(t *testing.T) {
	srv, root := newTestServer(t)
	rel := writeRootBench(t, root, `prompt: imported prompt
app: ./app
configs:
  - name: imported-cfg
    bundle: ./config-baseline
`)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/new?import="+url.QueryEscape(rel), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "imported prompt") || !strings.Contains(body, `value="imported-cfg"`) {
		t.Errorf("imported form should carry prompt and config name; body:\n%s", body)
	}
	wantApp := `value="` + filepath.Join(root, "app") + `"`
	if !strings.Contains(body, wantApp) {
		t.Errorf("imported form should carry the absolute app path %q; body:\n%s", wantApp, body)
	}
	wantBundle := `value="` + filepath.Join(root, "config-baseline") + `"`
	if !strings.Contains(body, wantBundle) {
		t.Errorf("imported form should carry the absolute bundle path %q; body:\n%s", wantBundle, body)
	}
}

func TestNewImportErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    string // written to root/bench.yaml; empty means no file
		imp     string // ?import= value; defaults to ./bench.yaml
		wantMsg string
	}{
		{name: "missing file", imp: "./nope.yaml", wantMsg: "fichier introuvable"},
		{name: "not a mapping", body: "- a\n- b\n", wantMsg: "mapping YAML attendu"},
		{name: "no configs", body: "app: ./app\n", wantMsg: "aucune configuration"},
		{name: "bad field type", body: "runs: abc\nconfigs:\n  - name: c\n    bundle: ./config-baseline\n", wantMsg: "invalide"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			srv, root := newTestServer(t)
			imp := tt.imp
			if imp == "" {
				imp = writeRootBench(t, root, tt.body)
			}

			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/new?import="+url.QueryEscape(imp), nil))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tt.wantMsg) {
				t.Errorf("body should mention %q; got:\n%s", tt.wantMsg, rec.Body.String())
			}
		})
	}
}

func TestNewWithoutParamsRendersDefaultForm(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/new", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestNewFromInvalidDirIsRejected(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/new?from="+url.QueryEscape("../../etc"), nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
