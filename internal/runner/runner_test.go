package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/keyxmare/claude-benchy/internal/docker"
	"github.com/keyxmare/claude-benchy/internal/spec"
)

// fakeDocker simulates a Claude run: it mutates the workspace and emits a
// stream-json result event, so the runner can be exercised without Docker.
type fakeDocker struct {
	changeFile string
}

func (f fakeDocker) Build(context.Context, string, string, map[string]string) error { return nil }

func (f fakeDocker) Run(_ context.Context, spec docker.RunSpec, stdout, _ io.Writer) error {
	if f.changeFile != "" {
		_ = os.WriteFile(filepath.Join(spec.WorkDir, f.changeFile), []byte("edited by claude\n"), 0o644)
	}
	fmt.Fprintln(stdout, `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]}}`)
	fmt.Fprintln(stdout, `{"type":"result","subtype":"success","is_error":false,"num_turns":2,"total_cost_usd":0.05,"result":"ok"}`)
	return nil
}

func newSpec(t *testing.T) *spec.Spec {
	t.Helper()
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "main.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(bundle, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "CLAUDE.md"), []byte("cfg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return &spec.Spec{
		App:         app,
		Output:      filepath.Join(base, "results"),
		Model:       "sonnet",
		Runs:        1,
		Concurrency: 1,
		Configs: []spec.Config{
			{Name: "a", Bundle: bundle, Prompt: "do the thing", Model: "sonnet"},
		},
	}
}

func TestRunEndToEndWithFake(t *testing.T) {
	s := newSpec(t)
	outputRoot := filepath.Join(s.Output, "ts")

	var logs []string
	rep, err := Run(context.Background(), s, outputRoot, "gen", Options{
		Image:  "img",
		Docker: fakeDocker{changeFile: "added.txt"},
		Log:    func(line string) { logs = append(logs, line) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(logs, func(l string) bool { return strings.HasPrefix(l, "✓") }) {
		t.Errorf("expected a completion progress line, got %v", logs)
	}
	if len(rep.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(rep.Runs))
	}
	r := rep.Runs[0]
	if r.Err != "" {
		t.Fatalf("unexpected run error: %s", r.Err)
	}
	if r.Metrics.NumTurns != 2 || r.Metrics.ToolUses != 1 {
		t.Errorf("metrics not parsed: %+v", r.Metrics)
	}
	if r.Diff.FilesChanged != 1 || r.Diff.Insertions != 1 {
		t.Errorf("diff not captured: %+v", r.Diff)
	}

	for _, name := range []string{"transcript.jsonl", "result.json", "diff.patch"} {
		if _, err := os.Stat(filepath.Join(outputRoot, "a", name)); err != nil {
			t.Errorf("missing artifact %s: %v", name, err)
		}
	}
}

func TestRunRecordsPrepareFailure(t *testing.T) {
	s := newSpec(t)
	s.Configs[0].Bundle = filepath.Join(t.TempDir(), "missing")
	rep, err := Run(context.Background(), s, filepath.Join(s.Output, "ts"), "gen", Options{
		Image:  "img",
		Docker: fakeDocker{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Runs[0].Err == "" {
		t.Error("expected recorded error for missing bundle")
	}
}
