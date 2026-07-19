package runner

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/report"
)

func TestBenchInfo(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		root := t.TempDir()
		writeJSON(filepath.Join(root, "bench.json"), benchMeta{Prompt: "do it", App: "/app"})
		prompt, app := BenchInfo(root)
		if prompt != "do it" || app != "/app" {
			t.Errorf("BenchInfo() = %q, %q; want %q, %q", prompt, app, "do it", "/app")
		}
	})
	t.Run("missing", func(t *testing.T) {
		prompt, app := BenchInfo(t.TempDir())
		if prompt != "" || app != "" {
			t.Errorf("BenchInfo() on empty dir = %q, %q; want empty", prompt, app)
		}
	})
}

func TestReloadRebuildsReportFromArtifacts(t *testing.T) {
	root := t.TempDir()

	// A complete run directory: meta, metrics, diff, checks and post-run workspace.
	dirA := filepath.Join(root, "a")
	if err := os.MkdirAll(filepath.Join(dirA, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(filepath.Join(dirA, "meta.json"), runMeta{Config: "a", Run: 1, Model: "sonnet", Bundle: "/bundle"})
	writeJSON(filepath.Join(dirA, "result.json"), claude.Metrics{NumTurns: 2, TotalCostUSD: 0.05, ToolUses: 1, Result: "ok"})
	writeJSON(filepath.Join(dirA, "checks.json"), []report.Check{{Name: "gate", Passed: true}})
	if err := os.WriteFile(filepath.Join(dirA, "diff.patch"), []byte("diff --git a/added.txt b/added.txt\n@@ -0,0 +1 @@\n+edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirA, "workspace", "added.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A minimal run directory nested under a config: only a diff, no meta / result
	// / checks, so the reconstructed run falls back to its relative path.
	dirB := filepath.Join(root, "b", "run-2")
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "diff.patch"), []byte("diff --git a/x.txt b/x.txt\n@@ -0,0 +1 @@\n+y\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeJSON(filepath.Join(root, "evaluation.json"), report.Evaluation{Model: "sonnet", Recommendation: "keep a"})

	rep, err := Reload(root, "/app", "the prompt", "gen")
	if err != nil {
		t.Fatal(err)
	}

	if rep.Prompt != "the prompt" || rep.App != "/app" || rep.GeneratedAt != "gen" {
		t.Errorf("report header = %q/%q/%q, want the passed-in values", rep.Prompt, rep.App, rep.GeneratedAt)
	}
	if len(rep.Runs) != 2 {
		t.Fatalf("expected 2 reloaded runs, got %d", len(rep.Runs))
	}
	// Sorted by ArtifactDir: "a" before "b/run-2".
	a := rep.Runs[0]
	if a.Config != "a" || a.Model != "sonnet" || a.Bundle != "/bundle" {
		t.Errorf("run a meta not reloaded: %+v", a)
	}
	if a.Metrics.NumTurns != 2 || a.Metrics.ToolUses != 1 {
		t.Errorf("run a metrics not reloaded: %+v", a.Metrics)
	}
	if a.Diff.FilesChanged != 1 {
		t.Errorf("run a diff stats not derived: %+v", a.Diff)
	}
	if len(a.Files) != 1 || a.Files[0].Path != "added.txt" || a.Files[0].Content != "edited\n" {
		t.Errorf("run a files not collected: %+v", a.Files)
	}
	if len(a.Checks) != 1 || !a.Checks[0].Passed {
		t.Errorf("run a checks not reloaded: %+v", a.Checks)
	}

	b := rep.Runs[1]
	if b.Config != filepath.Join("b", "run-2") {
		t.Errorf("run without meta should fall back to its rel path, got %q", b.Config)
	}
	if b.Metrics.NumTurns != 0 || len(b.Checks) != 0 {
		t.Errorf("run without result/checks should stay zero-valued: %+v", b)
	}

	if rep.Evaluation == nil || rep.Evaluation.Model != "sonnet" {
		t.Errorf("evaluation not reloaded: %+v", rep.Evaluation)
	}
}

func TestReloadWithoutEvaluation(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "a")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "diff.patch"), []byte("diff --git a/x b/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Reload(root, "/app", "p", "gen")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Evaluation != nil {
		t.Errorf("expected no evaluation when evaluation.json is absent, got %+v", rep.Evaluation)
	}
}

func TestReloadReportsWalkError(t *testing.T) {
	// A non-existent root makes WalkDir invoke the callback with an error.
	_, err := Reload(filepath.Join(t.TempDir(), "does-not-exist"), "/app", "p", "gen")
	if err == nil {
		t.Error("expected an error when the output root cannot be walked")
	}
}

func TestReadEmbed(t *testing.T) {
	dir := t.TempDir()

	big := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(big, bytes.Repeat([]byte("a"), maxEmbedBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "bin")
	if err := os.WriteFile(binary, []byte{0xff, 0xfe, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}
	normal := filepath.Join(dir, "n.txt")
	if err := os.WriteFile(normal, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct {
		path      string
		wantExact string
		wantHas   string
	}{
		"too-large": {big, "", "trop volumineux"},
		"binary":    {binary, "… contenu binaire non affiché\n", ""},
		"normal":    {normal, "hi\n", ""},
		"directory": {sub, "", ""},
		"missing":   {filepath.Join(dir, "nope"), "", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := readEmbed(tc.path)
			switch {
			case tc.wantHas != "":
				if !bytes.Contains([]byte(got), []byte(tc.wantHas)) {
					t.Errorf("readEmbed(%s) = %q, want it to contain %q", name, got, tc.wantHas)
				}
			default:
				if got != tc.wantExact {
					t.Errorf("readEmbed(%s) = %q, want %q", name, got, tc.wantExact)
				}
			}
		})
	}
}

func TestCollectFilesSkipsDuplicateAndMalformedHeaders(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "x.txt"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := "diff --git a/x.txt b/x.txt\n@@ -0,0 +1 @@\n+content\n" +
		"diff --git a/x.txt b/x.txt\n@@ -0,0 +1 @@\n+again\n" + // duplicate path: skipped
		"diff --git malformed header\n" // no " b/": skipped

	files := collectFiles(ws, patch)
	if len(files) != 1 || files[0].Path != "x.txt" {
		t.Errorf("collectFiles = %+v, want a single x.txt entry", files)
	}
}
