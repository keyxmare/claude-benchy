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

func TestPrepareGitAppWithoutCommitsFails(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	dst := filepath.Join(base, "ws")

	write(t, filepath.Join(app, "tracked.go"), "package app")
	gitInit(t, app) // .git exists but HEAD has no commit
	write(t, filepath.Join(bundle, "CLAUDE.md"), "cfg")

	err := workspace.Prepare(app, bundle, dst, false)

	if err == nil {
		t.Fatal("Prepare should fail when the git app has no HEAD commit to archive")
	}
}

func TestPrepareEmptyTreesFailBaseline(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	dst := filepath.Join(base, "ws")
	mkdir(t, app)
	mkdir(t, bundle)

	err := workspace.Prepare(app, bundle, dst, false)

	if err == nil {
		t.Fatal("Prepare should fail: an empty workspace has nothing to commit as baseline")
	}
}

func TestPrepareMkdirDstFails(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	mkdir(t, app)
	mkdir(t, bundle)
	// A regular file where dst's parent is expected makes MkdirAll(dst) fail.
	blocker := filepath.Join(base, "blocker")
	write(t, blocker, "x")
	dst := filepath.Join(blocker, "ws")

	err := workspace.Prepare(app, bundle, dst, false)

	if err == nil {
		t.Fatal("Prepare should fail when dst cannot be created")
	}
}

func TestPrepareMissingAppFails(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	app := filepath.Join(base, "missing")
	bundle := filepath.Join(base, "bundle")
	dst := filepath.Join(base, "ws")
	mkdir(t, bundle)

	err := workspace.Prepare(app, bundle, dst, false)

	if err == nil {
		t.Fatal("Prepare should fail when the app tree does not exist")
	}
}

func TestPrepareMissingBundleFails(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "missing")
	dst := filepath.Join(base, "ws")
	write(t, filepath.Join(app, "main.go"), "package main")

	err := workspace.Prepare(app, bundle, dst, false)

	if err == nil {
		t.Fatal("Prepare should fail when the bundle tree does not exist")
	}
}

func TestPrepareCopiesSymlinks(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	dst := filepath.Join(base, "ws")

	write(t, filepath.Join(app, "main.go"), "package main")
	write(t, filepath.Join(bundle, "CLAUDE.md"), "cfg")
	if err := os.Symlink("CLAUDE.md", filepath.Join(bundle, "link")); err != nil {
		t.Fatal(err)
	}

	if err := workspace.Prepare(app, bundle, dst, false); err != nil {
		t.Fatal(err)
	}

	got, err := os.Readlink(filepath.Join(dst, "link"))
	if err != nil {
		t.Fatalf("symlink not overlaid: %v", err)
	}
	if got != "CLAUDE.md" {
		t.Errorf("symlink target = %q, want %q", got, "CLAUDE.md")
	}
}

func TestPrepareSkipsBundleGitDir(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	dst := filepath.Join(base, "ws")

	write(t, filepath.Join(app, "main.go"), "package main")
	write(t, filepath.Join(bundle, "CLAUDE.md"), "cfg")
	write(t, filepath.Join(bundle, ".git", "MARKER"), "bundle vcs internals")

	if err := workspace.Prepare(app, bundle, dst, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dst, ".git", "MARKER")); err == nil {
		t.Error("bundle .git contents must not be copied into the workspace")
	}
}

func TestPrepareMergeOntoDirClaudeMdFails(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	dst := filepath.Join(base, "ws")

	// The app carries a *directory* named CLAUDE.md; appending the bundle's
	// CLAUDE.md file onto it cannot open the target for appending.
	write(t, filepath.Join(app, "CLAUDE.md", "inner.txt"), "dir masquerading as file")
	write(t, filepath.Join(bundle, "CLAUDE.md"), "overlay")

	err := workspace.Prepare(app, bundle, dst, true)

	if err == nil {
		t.Fatal("Prepare should fail when merging CLAUDE.md onto a directory")
	}
}

func TestPrepareCopyOntoDirFails(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	app := filepath.Join(base, "app")
	bundle := filepath.Join(base, "bundle")
	dst := filepath.Join(base, "ws")

	// The app has a directory "conf"; the bundle has a file "conf" that would
	// have to be written over the directory.
	write(t, filepath.Join(app, "conf", "inner.txt"), "dir")
	write(t, filepath.Join(bundle, "conf"), "file")

	err := workspace.Prepare(app, bundle, dst, false)

	if err == nil {
		t.Fatal("Prepare should fail when a bundle file collides with an app directory")
	}
}

// The following branches of workspace.go are intentionally left uncovered:
// they are error paths that cannot be reached deterministically in this test
// suite, which runs as root inside the toolchain container.
//
//   - gitArchive untar.Start (l.68): exec Start fails only if `tar` is absent
//     from PATH, which never happens in the toolchain image.
//   - gitArchive untar.Wait / "tar extract" (l.77): dst is freshly created and
//     empty before extraction and a valid HEAD tar always extracts; only a
//     mid-stream IO failure would trip it.
//   - copyTree filepath.Rel (l.114): path is always a descendant of
//     root = Clean(src), so Rel never errors (it only does across volumes).
//   - copyTree d.Info for a directory (l.129): the lazy re-stat fails only if
//     the directory vanishes mid-walk (a race).
//   - copyTree os.Readlink (l.135): the entry is already known to be a symlink,
//     so Readlink fails only on an IO error.
//   - appendFile os.ReadFile (l.158), copyFile os.Stat (l.175) and
//     copyFile os.Open (l.182): src is a regular file just enumerated by
//     WalkDir; only a permission/IO error triggers these, unreachable as root.
//   - appendFile WriteString (l.166) and copyFile io.Copy (l.191): the target
//     is already open for writing; these fail only on a mid-stream IO error.
//   - copyFile os.MkdirAll (l.178): WalkDir visits every parent directory
//     before its children, so the dir case already created dst's parent; this
//     defensive MkdirAll only ever re-creates an existing directory.

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", dir, "init", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
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
