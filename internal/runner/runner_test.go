package runner

import (
	"context"
	"encoding/json"
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

// fakeDocker simulates the sandbox: a Claude run mutates the workspace and emits
// a stream-json result event; a check run (entrypoint override) passes unless
// failCheck is set; the judge run (its prompt mentions the evaluator role)
// returns a canned evaluation JSON. This lets the runner be exercised end to
// end without Docker.
type fakeDocker struct {
	changeFile string
	failCheck  bool
}

func (f fakeDocker) Build(context.Context, string, string, string, map[string]string) error {
	return nil
}

func (f fakeDocker) Run(_ context.Context, spec docker.RunSpec, stdout, _ io.Writer) error {
	if spec.Entrypoint != "" {
		if f.failCheck {
			return fmt.Errorf("check command failed")
		}
		return nil
	}

	if strings.Contains(promptArg(spec.Args), "évaluateur") {
		answer := `{"configs":[{"label":"a","verdict":"Répond bien","criteria":[{"criterion":"couvre le cas nominal","level":"respecté","note":"ok"}]}],"recommendation":"RAS"}`
		evt, _ := json.Marshal(map[string]any{
			"type": "result", "subtype": "success", "is_error": false, "result": answer,
		})
		_, _ = stdout.Write(append(evt, '\n'))
		return nil
	}

	if f.changeFile != "" {
		_ = os.WriteFile(filepath.Join(spec.WorkDir, f.changeFile), []byte("edited by claude\n"), 0o644)
	}
	_, _ = fmt.Fprintln(stdout, `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]}}`)
	_, _ = fmt.Fprintln(stdout, `{"type":"result","subtype":"success","is_error":false,"num_turns":2,"total_cost_usd":0.05,"result":"ok"}`)
	return nil
}

func promptArg(args []string) string {
	for i, a := range args {
		if a == "-p" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
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
	var events []AgentEvent
	rep, err := Run(context.Background(), s, outputRoot, "gen", Options{
		Image:  "img",
		Docker: fakeDocker{changeFile: "added.txt"},
		Log:    func(line string) { logs = append(logs, line) },
		Agent:  func(ev AgentEvent) { events = append(events, ev) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(logs, func(l string) bool { return strings.HasPrefix(l, "✓") }) {
		t.Errorf("expected a completion progress line, got %v", logs)
	}
	hasAgent := func(pred func(AgentEvent) bool) bool { return slices.ContainsFunc(events, pred) }
	if !hasAgent(func(e AgentEvent) bool { return e.Agent == "a" && e.Status == "running" }) {
		t.Errorf("expected a running status event for agent a, got %+v", events)
	}
	if !hasAgent(func(e AgentEvent) bool { return e.Agent == "a" && strings.HasPrefix(e.Line, "✏️ Write") }) {
		t.Errorf("expected a rendered tool-use line for agent a, got %+v", events)
	}
	if !hasAgent(func(e AgentEvent) bool { return e.Agent == "a" && e.Status == "done" }) {
		t.Errorf("expected a done status event for agent a, got %+v", events)
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
	if len(r.Files) != 1 || r.Files[0].Path != "added.txt" || r.Files[0].Content != "edited by claude\n" {
		t.Errorf("touched file content not collected: %+v", r.Files)
	}

	for _, name := range []string{"transcript.jsonl", "result.json", "diff.patch"} {
		if _, err := os.Stat(filepath.Join(outputRoot, "a", name)); err != nil {
			t.Errorf("missing artifact %s: %v", name, err)
		}
	}
}

func TestRunWithEvaluationJudgesAndChecks(t *testing.T) {
	s := newSpec(t)
	s.Evaluate = spec.Evaluate{
		Model:  "sonnet",
		Rubric: []string{"couvre le cas nominal"},
		Checks: []spec.Check{
			{Name: "fichier attendu", File: "added.txt"},
			{Name: "gate", Run: "true"},
		},
	}
	outputRoot := filepath.Join(s.Output, "ts")

	rep, err := Run(context.Background(), s, outputRoot, "gen", Options{
		Image:  "img",
		Docker: fakeDocker{changeFile: "added.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if rep.Evaluation == nil {
		t.Fatal("expected an evaluation")
	}
	if rep.Evaluation.Model != "sonnet" {
		t.Errorf("judge model not recorded: %q", rep.Evaluation.Model)
	}
	if len(rep.Evaluation.Rubric) != 1 {
		t.Errorf("rubric not carried through: %v", rep.Evaluation.Rubric)
	}
	if len(rep.Evaluation.Ranking) != 1 || rep.Evaluation.Ranking[0] != "a" {
		t.Errorf("ranking not parsed: %v", rep.Evaluation.Ranking)
	}
	if len(rep.Evaluation.Configs) != 1 || rep.Evaluation.Configs[0].Score != 100 {
		t.Errorf("config eval not parsed / scored: %+v", rep.Evaluation.Configs)
	}
	if got := rep.Evaluation.Configs[0].Level("couvre le cas nominal"); got != "respecté" {
		t.Errorf("criterion level not parsed: %q", got)
	}

	checks := rep.Runs[0].Checks
	if len(checks) != 2 || !checks[0].Passed || !checks[1].Passed {
		t.Errorf("checks not run/collected as expected: %+v", checks)
	}

	for _, name := range []string{"checks.json"} {
		if _, err := os.Stat(filepath.Join(outputRoot, "a", name)); err != nil {
			t.Errorf("missing per-config artifact %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outputRoot, "evaluation.json")); err != nil {
		t.Errorf("missing evaluation.json: %v", err)
	}
}

func TestRunEvaluationRecordsFailingCheck(t *testing.T) {
	s := newSpec(t)
	s.Evaluate = spec.Evaluate{Checks: []spec.Check{{Name: "gate", Run: "false"}}}
	rep, err := Run(context.Background(), s, filepath.Join(s.Output, "ts"), "gen", Options{
		Image:  "img",
		Docker: fakeDocker{changeFile: "added.txt", failCheck: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	checks := rep.Runs[0].Checks
	if len(checks) != 1 || checks[0].Passed {
		t.Errorf("expected a failing check, got %+v", checks)
	}
}

func TestPostImagePath(t *testing.T) {
	cases := map[string]struct {
		line string
		want string
		ok   bool
	}{
		"edit":      {"diff --git a/internal/x.go b/internal/x.go", "internal/x.go", true},
		"nested":    {"diff --git a/a/b/c.go b/a/b/c.go", "a/b/c.go", true},
		"unrelated": {"@@ -1,2 +1,3 @@", "", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := postImagePath(tc.line)
			if ok != tc.ok || got != tc.want {
				t.Errorf("postImagePath(%q) = %q, %v; want %q, %v", tc.line, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestCollectFilesReadsTouchedFilesOnly(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "touched.txt"), []byte("new content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "ignored.txt"), []byte("untouched\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := "diff --git a/touched.txt b/touched.txt\n@@ -0,0 +1 @@\n+new content\n" +
		"diff --git a/gone.txt b/gone.txt\ndeleted file mode 100644\n"

	files := collectFiles(ws, patch)
	if len(files) != 2 {
		t.Fatalf("expected 2 touched files, got %d: %+v", len(files), files)
	}
	// Sorted by path: gone.txt (deleted → empty), touched.txt.
	if files[0].Path != "gone.txt" || files[0].Content != "" {
		t.Errorf("deleted file should yield empty content, got %+v", files[0])
	}
	if files[1].Path != "touched.txt" || files[1].Content != "new content\n" {
		t.Errorf("touched file content wrong, got %+v", files[1])
	}
}

// flakyDocker emits a degenerate result (no tool call, no file change) for the
// first degenerateAttempts Claude runs, then a normal one — modelling a run
// that flubs the first time and succeeds on retry.
type flakyDocker struct {
	changeFile         string
	degenerateAttempts int
	claudeCalls        int
}

func (f *flakyDocker) Build(context.Context, string, string, string, map[string]string) error {
	return nil
}

func (f *flakyDocker) Run(_ context.Context, spec docker.RunSpec, stdout, _ io.Writer) error {
	if spec.Entrypoint != "" {
		return nil
	}
	f.claudeCalls++
	if f.claudeCalls <= f.degenerateAttempts {
		// No tool_use event and no file written: a degenerate, no-op run.
		_, _ = fmt.Fprintln(stdout, `{"type":"result","subtype":"success","is_error":false,"num_turns":1,"total_cost_usd":0.01,"result":"Agent({...}) written as text"}`)
		return nil
	}
	_ = os.WriteFile(filepath.Join(spec.WorkDir, f.changeFile), []byte("edited by claude\n"), 0o644)
	_, _ = fmt.Fprintln(stdout, `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write"}]}}`)
	_, _ = fmt.Fprintln(stdout, `{"type":"result","subtype":"success","is_error":false,"num_turns":2,"total_cost_usd":0.05,"result":"ok"}`)
	return nil
}

func TestRunRetriesDegenerateRunUntilItProducesWork(t *testing.T) {
	s := newSpec(t)
	retries := 2
	s.Retries = &retries

	fake := &flakyDocker{changeFile: "added.txt", degenerateAttempts: 1}
	var logs []string
	rep, err := Run(context.Background(), s, filepath.Join(s.Output, "ts"), "gen", Options{
		Image: "img", Docker: fake, Log: func(l string) { logs = append(logs, l) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.claudeCalls != 2 {
		t.Fatalf("expected one retry after a degenerate run, got %d Claude call(s)", fake.claudeCalls)
	}
	r := rep.Runs[0]
	if !r.OK() || r.Degenerate() {
		t.Errorf("retried run should be OK and not degenerate: %+v", r.Metrics)
	}
	if r.Diff.FilesChanged != 1 {
		t.Errorf("expected the successful attempt's diff, got %+v", r.Diff)
	}
	if !slices.ContainsFunc(logs, func(l string) bool { return strings.HasPrefix(l, "↻") }) {
		t.Errorf("expected a retry progress line, got %v", logs)
	}
}

func TestRunRecordsPersistentlyDegenerateRun(t *testing.T) {
	s := newSpec(t)
	zero := 0
	s.Retries = &zero // no retries: a single degenerate attempt is recorded as-is

	fake := &flakyDocker{changeFile: "added.txt", degenerateAttempts: 5}
	rep, err := Run(context.Background(), s, filepath.Join(s.Output, "ts"), "gen", Options{
		Image: "img", Docker: fake,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.claudeCalls != 1 {
		t.Fatalf("with retries=0 expected a single attempt, got %d", fake.claudeCalls)
	}
	r := rep.Runs[0]
	if !r.Degenerate() {
		t.Errorf("expected a degenerate run, got %+v", r.Metrics)
	}
	if r.OK() {
		t.Error("a degenerate run must not count as OK")
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

func TestClaudeArgsForceNonInteractive(t *testing.T) {
	args := claudeArgs(spec.Config{Name: "vanilla", Prompt: "Documente ce projet Go.", Model: "sonnet"})

	if got := promptArg(args); got != "Documente ce projet Go." {
		t.Errorf("promptArg(args) = %q, want the prompt right after -p", got)
	}
	if got := flagValue(args, "--append-system-prompt"); got != headlessSystemPrompt {
		t.Errorf("flagValue(--append-system-prompt) = %q, want the headless instruction", got)
	}
	if got := flagValue(args, "--model"); got != "sonnet" {
		t.Errorf("flagValue(--model) = %q, want %q", got, "sonnet")
	}
}

// flagValue returns the argument following flag, or "" when absent.
func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
