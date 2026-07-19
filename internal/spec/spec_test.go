package spec_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/keyxmare/claude-benchy/internal/spec"
)

func writeSpec(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	mustDir(t, filepath.Join(dir, "app"))
	mustDir(t, filepath.Join(dir, "cfg-a"))
	mustDir(t, filepath.Join(dir, "cfg-b"))
	path := filepath.Join(dir, "bench.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustDir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefaultsAndOverrides(t *testing.T) {
	path := writeSpec(t, `
prompt: base prompt
app: ./app
configs:
  - name: a
    bundle: ./cfg-a
  - name: b
    bundle: ./cfg-b
    model: opus
    prompt: override
`)
	s, err := spec.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Model != "sonnet" || s.Runs != 1 || s.Concurrency != 1 {
		t.Errorf("defaults not applied: %+v", s)
	}
	if !filepath.IsAbs(s.App) {
		t.Errorf("app not resolved to abs: %s", s.App)
	}
	if got := s.Configs[0]; got.Prompt != "base prompt" || got.Model != "sonnet" {
		t.Errorf("config a resolution wrong: %+v", got)
	}
	if got := s.Configs[1]; got.Prompt != "override" || got.Model != "opus" {
		t.Errorf("config b resolution wrong: %+v", got)
	}
	if filepath.Base(filepath.Dir(s.CredsFile)) == "" || filepath.Base(s.CredsFile) != ".credentials.json" {
		t.Errorf("creds file wrong: %s", s.CredsFile)
	}
}

func TestLoadRetries(t *testing.T) {
	cases := map[string]struct {
		field string
		want  int
	}{
		"unset defaults":  {"", 2},
		"explicit zero":   {"retries: 0", 0},
		"explicit number": {"retries: 3", 3},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s, err := spec.Load(writeSpec(t, `
prompt: p
app: ./app
`+tc.field+`
configs:
  - name: a
    bundle: ./cfg-a
`))
			if err != nil {
				t.Fatal(err)
			}
			if got := s.RetryCount(); got != tc.want {
				t.Errorf("RetryCount() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestLoadPromptFile(t *testing.T) {
	path := writeSpec(t, `
promptFile: ./prompt.txt
app: ./app
configs:
  - name: a
    bundle: ./cfg-a
`)
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "prompt.txt"), []byte("from file"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := spec.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Configs[0].Prompt != "from file" {
		t.Errorf("promptFile not read: %q", s.Configs[0].Prompt)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := map[string]string{
		"no prompt": `
app: ./app
configs:
  - name: a
    bundle: ./cfg-a
`,
		"duplicate name": `
prompt: p
app: ./app
configs:
  - name: a
    bundle: ./cfg-a
  - name: a
    bundle: ./cfg-b
`,
		"prompt and file": `
prompt: p
promptFile: ./x.txt
app: ./app
configs:
  - name: a
    bundle: ./cfg-a
`,
		"no configs": `
prompt: p
app: ./app
`,
		"unknown field": `
prompt: p
app: ./app
bogus: 1
configs:
  - name: a
    bundle: ./cfg-a
`,
		"negative retries": `
prompt: p
app: ./app
retries: -1
configs:
  - name: a
    bundle: ./cfg-a
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := spec.Load(writeSpec(t, body)); err == nil {
				t.Errorf("expected error for %q", name)
			}
		})
	}
}

func TestLoadEvaluate(t *testing.T) {
	path := writeSpec(t, `
prompt: p
app: ./app
model: opus
evaluate:
  rubric:
    - couvre le cas nominal
  checks:
    - name: tests
      run: go test ./...
    - name: fichier
      file: internal/x_test.go
configs:
  - name: a
    bundle: ./cfg-a
`)
	s, err := spec.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Evaluate.Enabled() {
		t.Fatal("evaluate should be enabled")
	}
	if s.Evaluate.Model != "opus" {
		t.Errorf("judge model should default to spec model, got %q", s.Evaluate.Model)
	}
	if len(s.Evaluate.Checks) != 2 || s.Evaluate.Checks[0].Run != "go test ./..." || s.Evaluate.Checks[1].File != "internal/x_test.go" {
		t.Errorf("checks not parsed: %+v", s.Evaluate.Checks)
	}
}

func TestLoadEvaluateInvalidCheck(t *testing.T) {
	path := writeSpec(t, `
prompt: p
app: ./app
evaluate:
  checks:
    - name: empty
configs:
  - name: a
    bundle: ./cfg-a
`)
	if _, err := spec.Load(path); err == nil {
		t.Error("expected error for a check with neither run nor file")
	}
}

func TestLoadMissingBundle(t *testing.T) {
	path := writeSpec(t, `
prompt: p
app: ./app
configs:
  - name: a
    bundle: ./does-not-exist
`)
	if _, err := spec.Load(path); err == nil {
		t.Error("expected error for missing bundle")
	}
}
