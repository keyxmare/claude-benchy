package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keyxmare/claude-benchy/internal/runner"
)

func intPtr(n int) *int { return &n }

func TestFormFromDocMapsEveryField(t *testing.T) {
	d := benchDoc{
		Prompt: "p", App: "a", Model: "opus",
		Runs: 3, Concurrency: 2, Retries: intPtr(5), KeepBaseConfig: true,
		Auth:     &authDoc{ConfigDir: "/cfg"},
		Sandbox:  &sandboxDoc{Image: "img"},
		Output:   "out",
		Evaluate: &evaluateDoc{Model: "haiku", Rubric: []string{"r1", "r2"}, Checks: []checkDoc{{Name: "c", Run: "go test", File: "f"}}},
		Configs:  []configDoc{{Name: "n", Bundle: "b", Model: "m", Prompt: "cp"}},
	}

	got := formFromDoc(d)

	want := formValues{
		Prompt: "p", App: "a", Model: "opus",
		Runs: "3", Concurrency: "2", Retries: "5",
		AuthConfigDir: "/cfg", SandboxImage: "img", Output: "out",
		EvalModel: "haiku", Rubric: "r1\nr2", KeepBaseConfig: true,
		Configs: []configRow{{Name: "n", Bundle: "b", Model: "m", Prompt: "cp"}},
		Checks:  []checkRow{{Name: "c", Run: "go test", File: "f"}},
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("formFromDoc mismatch (-got +want):\n%s", diff)
	}
}

func TestGuessBundle(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "configs", "here"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "configs", "afile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct {
		name string
		want string
	}{
		"existing dir": {"here", "./configs/here"},
		"a file":       {"afile", ""},
		"missing":      {"gone", ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := guessBundle(root, tt.name); got != tt.want {
				t.Errorf("guessBundle(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestConfigMeta(t *testing.T) {
	root := t.TempDir()

	direct := filepath.Join(root, "direct")
	if err := os.MkdirAll(direct, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(direct, "meta.json"), []byte(`{"config":"c","model":"opus"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(filepath.Join(nested, "run-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "loose.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "run-1", "meta.json"), []byte(`{"config":"n","model":"haiku"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	empty := filepath.Join(root, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		dir              string
		wantN, wantModel string
	}{
		"direct meta":  {direct, "c", "opus"},
		"nested meta":  {nested, "n", "haiku"},
		"no meta":      {empty, "", ""},
		"missing path": {filepath.Join(root, "gone"), "", ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			n, m := configMeta(tt.dir)
			if n != tt.wantN || m != tt.wantModel {
				t.Errorf("configMeta(%q) = (%q, %q), want (%q, %q)", tt.dir, n, m, tt.wantN, tt.wantModel)
			}
		})
	}
}

func TestReadBenchDoc(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) string {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if body != "" {
			if err := os.WriteFile(filepath.Join(dir, "bench.yaml"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}

	tests := map[string]struct {
		dir    string
		wantOK bool
	}{
		"missing file": {write("missing", ""), false},
		"invalid yaml": {write("invalid", "\tnot: [valid"), false},
		"no configs":   {write("noconfigs", "app: a\n"), false},
		"valid":        {write("valid", "app: a\nconfigs:\n  - name: c\n    bundle: b\n"), true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, ok := readBenchDoc(tt.dir)
			if ok != tt.wantOK {
				t.Errorf("readBenchDoc(%q) ok = %v, want %v", tt.dir, ok, tt.wantOK)
			}
		})
	}
}

func TestNearestBench(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a", "b", "c"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "bench.yaml"), []byte("app: x\nconfigs:\n  - name: c\n    bundle: b\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("found in ancestor", func(t *testing.T) {
		if _, ok := nearestBench(filepath.Join(root, "a", "b", "c")); !ok {
			t.Error("nearestBench should find the ancestor bench.yaml")
		}
	})

	t.Run("absent up to the filesystem root", func(t *testing.T) {
		// "/<unique>" reaches "/" within two steps, exercising the parent==cur
		// break without depending on the temp dir's depth.
		if _, ok := nearestBench("/benchy-nonexistent-parent"); ok {
			t.Error("nearestBench should report no bench when none exists up to the root")
		}
	})
}

func TestFormFromDirPrefersOwnBenchThenArtifacts(t *testing.T) {
	root := t.TempDir()

	own := filepath.Join(root, "own")
	if err := os.MkdirAll(own, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(own, "bench.yaml"), []byte("prompt: from-own\napp: a\nconfigs:\n  - name: c\n    bundle: b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := formFromDir(root, own).Prompt; got != "from-own" {
		t.Errorf("formFromDir prompt = %q, want the run's own bench.yaml", got)
	}

	artifacts := filepath.Join(root, "orphan", "run")
	if err := os.MkdirAll(artifacts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifacts, "bench.json"), []byte(`{"prompt":"from-artifacts","app":"/x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := formFromDir(root, artifacts).Prompt; got != "from-artifacts" {
		t.Errorf("formFromDir prompt = %q, want the artifact reconstruction", got)
	}
}

func TestFormFromArtifacts(t *testing.T) {
	root := t.TempDir()
	// A bundle exists for "baseline" (a dir) but "nested" maps to a file, and
	// "plain" has none — covering every guessBundle branch.
	if err := os.MkdirAll(filepath.Join(root, "configs", "baseline"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "configs", "nested"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(root, "results", "run")
	mkdir := func(parts ...string) string {
		p := filepath.Join(append([]string{dir}, parts...)...)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	writeFile := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkdir()
	writeFile(filepath.Join(dir, "bench.json"), `{"prompt":"do it","app":"/x/app"}`)
	writeFile(filepath.Join(mkdir("baseline"), "meta.json"), `{"config":"baseline","model":"opus"}`)
	mkdir("nested", "run-1")
	writeFile(filepath.Join(dir, "nested", "run-1", "meta.json"), `{"config":"nested","model":"haiku"}`)
	mkdir("plain")
	mkdir(".judge") // scratch dir, must be ignored
	writeFile(filepath.Join(dir, "evaluation.json"), `{"rubric":["r1","r2"]}`)

	got := formFromArtifacts(root, dir)

	if got.Prompt != "do it" || got.App != "/x/app" {
		t.Errorf("prompt/app = %q/%q, want %q/%q", got.Prompt, got.App, "do it", "/x/app")
	}
	if got.Rubric != "r1\nr2" {
		t.Errorf("rubric = %q, want %q", got.Rubric, "r1\nr2")
	}
	want := []configRow{
		{Name: "baseline", Bundle: "./configs/baseline", Model: "opus"},
		{Name: "nested", Bundle: "", Model: "haiku"},
		{Name: "plain", Bundle: "", Model: ""},
	}
	if diff := cmp.Diff(got.Configs, want); diff != "" {
		t.Errorf("configs mismatch (-got +want):\n%s", diff)
	}
}

func TestFormFromArtifactsDegradedInputs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "run")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A "varies per config" prompt is dropped; an unparsable evaluation.json
	// leaves the rubric empty; no config dirs leave the defaulted single row.
	if err := os.WriteFile(filepath.Join(dir, "bench.json"), []byte(`{"prompt":`+jsonQuote(runner.PromptVaries)+`,"app":"/y"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evaluation.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := formFromArtifacts(root, dir)

	if got.Prompt != "" {
		t.Errorf("prompt = %q, want empty for a varies-per-config bench", got.Prompt)
	}
	if got.App != "/y" || got.Rubric != "" {
		t.Errorf("app/rubric = %q/%q, want /y and empty", got.App, got.Rubric)
	}
	if diff := cmp.Diff(got.Configs, []configRow{{}}); diff != "" {
		t.Errorf("configs mismatch (-got +want):\n%s", diff)
	}
}

func jsonQuote(s string) string { return `"` + s + `"` }
