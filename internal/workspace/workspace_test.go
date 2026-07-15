package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPrepareOverlaysAndCommits(t *testing.T) {
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	dst := filepath.Join(base, "ws")

	write(t, filepath.Join(app, "main.go"), "package main")
	write(t, filepath.Join(app, "CLAUDE.md"), "old")
	write(t, filepath.Join(bundle, "CLAUDE.md"), "new")
	write(t, filepath.Join(bundle, ".claude", "skills", "s.md"), "skill")

	if err := Prepare(app, bundle, dst); err != nil {
		t.Fatal(err)
	}

	if got := read(t, filepath.Join(dst, "CLAUDE.md")); got != "new" {
		t.Errorf("bundle should overlay app CLAUDE.md, got %q", got)
	}
	if read(t, filepath.Join(dst, "main.go")) != "package main" {
		t.Error("app file missing")
	}
	if _, err := os.Stat(filepath.Join(dst, ".claude", "skills", "s.md")); err != nil {
		t.Error("bundle skill not overlaid")
	}

	// Baseline commit must exist and leave a clean tree.
	out, err := exec.Command("git", "-C", dst, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v: %s", err, out)
	}
	if len(out) != 0 {
		t.Errorf("tree not clean after baseline: %s", out)
	}
}

func TestPrepareSkipsSourceGit(t *testing.T) {
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	dst := filepath.Join(base, "ws")
	write(t, filepath.Join(app, "f.txt"), "x")
	write(t, filepath.Join(app, ".git", "config"), "should-not-copy")
	write(t, filepath.Join(bundle, "CLAUDE.md"), "c")

	if err := Prepare(app, bundle, dst); err != nil {
		t.Fatal(err)
	}
	if data := read(t, filepath.Join(dst, ".git", "config")); data == "should-not-copy" {
		t.Error("source .git was copied into workspace")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}
