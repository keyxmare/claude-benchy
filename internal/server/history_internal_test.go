package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func writeFileAt(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCountConfigs(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"cfgA", "cfgB", ".judge"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "bench.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := countConfigs(dir); got != 2 {
		t.Errorf("countConfigs = %d, want 2 (judge dir and files excluded)", got)
	}
	if got := countConfigs(filepath.Join(dir, "does-not-exist")); got != 0 {
		t.Errorf("countConfigs(missing) = %d, want 0", got)
	}
}

func TestScanHistorySkipsNoiseAndOrders(t *testing.T) {
	root := t.TempDir()

	// A fully described run, with an evaluation and one config directory.
	run1 := filepath.Join(root, "run1")
	writeFileAt(t, filepath.Join(run1, "bench.json"), `{"prompt":"P","app":"A"}`)
	writeFileAt(t, filepath.Join(run1, "bench.yaml"), "model: opus\nruns: 2\nconfigs:\n  - name: baseline\n")
	writeFileAt(t, filepath.Join(run1, "evaluation.json"), "{}")
	if err := os.MkdirAll(filepath.Join(run1, "baseline"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Two runs sharing a timestamp basename, to exercise the tie-break on Dir.
	writeFileAt(t, filepath.Join(root, "a", "20260101-000000", "bench.json"), "{}")
	writeFileAt(t, filepath.Join(root, "b", "20260101-000000", "bench.json"), "{}")

	// Pruned subtrees: a bench.json under .git or workspace must never surface.
	writeFileAt(t, filepath.Join(root, ".git", "bench.json"), "{}")
	writeFileAt(t, filepath.Join(root, "workspace", "bench.json"), "{}")

	got := scanHistory(root)

	var dirs []string
	for _, e := range got {
		dirs = append(dirs, e.Dir)
	}
	want := []string{"run1", filepath.Join("a", "20260101-000000"), filepath.Join("b", "20260101-000000")}
	if diff := cmp.Diff(dirs, want); diff != "" {
		t.Errorf("history order/pruning mismatch (-got +want):\n%s", diff)
	}

	first := got[0]
	if !first.HasEval || first.Model != "opus" || first.Runs != 2 {
		t.Errorf("run1 entry = %+v, want eval/opus/runs=2", first)
	}
	if len(first.Configs) != 1 || first.Configs[0] != "baseline" || first.Count != 1 {
		t.Errorf("run1 configs = %v count = %d, want [baseline]/1", first.Configs, first.Count)
	}
}
