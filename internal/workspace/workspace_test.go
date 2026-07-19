package workspace_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/keyxmare/claude-benchy/internal/workspace"
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

	if err := workspace.Prepare(app, bundle, dst, false); err != nil {
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

func TestPrepareGitAppUsesCommittedTree(t *testing.T) {
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	dst := filepath.Join(base, "ws")

	write(t, filepath.Join(app, "tracked.go"), "package app")
	write(t, filepath.Join(app, ".gitignore"), "build/\n")
	write(t, filepath.Join(app, "build", "artifact.bin"), "junk")
	commitApp(t, app)
	// Untracked file created after the commit must not reach the workspace.
	write(t, filepath.Join(app, "scratch.txt"), "uncommitted")
	write(t, filepath.Join(bundle, "CLAUDE.md"), "cfg")

	if err := workspace.Prepare(app, bundle, dst, false); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dst, "tracked.go")) != "package app" {
		t.Error("tracked file missing from workspace")
	}
	if read(t, filepath.Join(dst, "CLAUDE.md")) != "cfg" {
		t.Error("bundle not overlaid")
	}
	if _, err := os.Stat(filepath.Join(dst, "build", "artifact.bin")); err == nil {
		t.Error("ignored build artifact leaked into workspace")
	}
	if _, err := os.Stat(filepath.Join(dst, "scratch.txt")); err == nil {
		t.Error("untracked file leaked into workspace")
	}
}

func TestPrepareMergesClaudeMdWhenAsked(t *testing.T) {
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	dst := filepath.Join(base, "ws")

	write(t, filepath.Join(app, "CLAUDE.md"), "base instructions")
	write(t, filepath.Join(bundle, "CLAUDE.md"), "overlay instructions")

	if err := workspace.Prepare(app, bundle, dst, true); err != nil {
		t.Fatal(err)
	}
	got := read(t, filepath.Join(dst, "CLAUDE.md"))
	if got != "base instructions\n\noverlay instructions" {
		t.Errorf("CLAUDE.md should keep the base and append the overlay, got %q", got)
	}
}

func TestPrepareMergeCreatesClaudeMdWhenAppHasNone(t *testing.T) {
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	dst := filepath.Join(base, "ws")

	write(t, filepath.Join(app, "main.go"), "package main")
	write(t, filepath.Join(bundle, "CLAUDE.md"), "only overlay")

	if err := workspace.Prepare(app, bundle, dst, true); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dst, "CLAUDE.md")); got != "only overlay" {
		t.Errorf("with no base CLAUDE.md the bundle's should be used verbatim, got %q", got)
	}
}

func commitApp(t *testing.T, dir string) {
	t.Helper()
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	for _, args := range [][]string{
		{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "--no-gpg-sign", "-m", "base"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
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
