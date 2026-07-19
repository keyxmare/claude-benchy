package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/keyxmare/claude-benchy/internal/spec"
)

func formRequest(body url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(body.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestReadFormParsesChecksAndDefaultsConfig(t *testing.T) {
	// Checks with no config rows: the config list falls back to a single blank
	// row and the shorter parallel slices default their missing cells to "".
	fv := readForm(formRequest(url.Values{
		"checkName": {"gate"},
		"checkRun":  {"go test"},
	}))

	if len(fv.Configs) != 1 || fv.Configs[0] != (configRow{}) {
		t.Errorf("configs = %+v, want a single blank row", fv.Configs)
	}
	if len(fv.Checks) != 1 || fv.Checks[0] != (checkRow{Name: "gate", Run: "go test", File: ""}) {
		t.Errorf("checks = %+v, want the parsed row with an empty file", fv.Checks)
	}
}

func TestReadFormPadsShortParallelSlices(t *testing.T) {
	fv := readForm(formRequest(url.Values{
		"configName":   {"a", "b"},
		"configBundle": {"x"}, // shorter than configName
	}))

	if len(fv.Configs) != 2 {
		t.Fatalf("configs = %+v, want 2 rows", fv.Configs)
	}
	if fv.Configs[1].Bundle != "" || fv.Configs[0].Bundle != "x" {
		t.Errorf("bundles = %q/%q, want x and an empty padded cell", fv.Configs[0].Bundle, fv.Configs[1].Bundle)
	}
}

func TestSpecRejectsInvalidNumericFields(t *testing.T) {
	tests := map[string]formValues{
		"concurrency": {App: "a", Runs: "1", Concurrency: "x"},
		"retries":     {App: "a", Runs: "1", Concurrency: "1", Retries: "x"},
	}
	for name, fv := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := fv.spec(); err == nil {
				t.Errorf("spec() error = nil, want an error for a bad %s", name)
			}
		})
	}
}

func TestBenchDocFromKeepsOptionalSections(t *testing.T) {
	tests := map[string]struct {
		eval      spec.Evaluate
		wantModel string
		wantCheck bool
	}{
		"model and checks": {
			eval:      spec.Evaluate{Model: "m", Checks: []spec.Check{{Name: "n", Run: "r", File: "f"}}},
			wantModel: "m", wantCheck: true,
		},
		"rubric only, no model": {
			eval:      spec.Evaluate{Rubric: []string{"crit"}},
			wantModel: "", wantCheck: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := spec.Spec{
				App:      "a",
				Auth:     spec.Auth{ConfigDir: "/c"},
				Sandbox:  spec.Sandbox{Image: "img"},
				Evaluate: tt.eval,
				Configs:  []spec.Config{{Name: "c", Bundle: "b"}},
			}

			d := benchDocFrom(s)

			if d.Auth == nil || d.Auth.ConfigDir != "/c" {
				t.Errorf("auth = %+v, want configDir /c", d.Auth)
			}
			if d.Sandbox == nil || d.Sandbox.Image != "img" {
				t.Errorf("sandbox = %+v, want image img", d.Sandbox)
			}
			if d.Evaluate == nil || d.Evaluate.Model != tt.wantModel {
				t.Fatalf("evaluate = %+v, want model %q", d.Evaluate, tt.wantModel)
			}
			if got := len(d.Evaluate.Checks) == 1; got != tt.wantCheck {
				t.Errorf("has check = %v, want %v", got, tt.wantCheck)
			}
		})
	}
}
