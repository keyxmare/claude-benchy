package diffcap_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keyxmare/claude-benchy/internal/diffcap"
)

// git runs a git command in dir with a deterministic identity, failing the test
// on error. It mirrors internal/server/apply_test.go's gitRun.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-C", dir,
		"-c", "user.email=t@example.com",
		"-c", "user.name=test",
		"-c", "commit.gpgsign=false",
	}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// write writes body to path, failing the test on error.
func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// initRepo creates a git repo in a temp dir with one committed file (app.txt)
// and returns its path.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "app.txt"), "one\n")
	git(t, dir, "init", "-q")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "base")
	return dir
}

func TestCapture(t *testing.T) {
	t.Run("captures staged changes", func(t *testing.T) {
		t.Parallel()
		dir := initRepo(t)
		write(t, filepath.Join(dir, "app.txt"), "one\ntwo\n")
		write(t, filepath.Join(dir, "new.txt"), "hello\n")

		got, err := diffcap.Capture(dir)

		if err != nil {
			t.Fatalf("Capture(%q) error = %v", dir, err)
		}
		wantStats := diffcap.Stats{FilesChanged: 2, Insertions: 2, Deletions: 0}
		if d := cmp.Diff(wantStats, got.Stats); d != "" {
			t.Errorf("Capture(%q).Stats mismatch (-want +got):\n%s", dir, d)
		}
		if !strings.Contains(got.Patch, "new.txt") || !strings.Contains(got.Patch, "+two") {
			t.Errorf("Capture(%q).Patch = %q, want it to mention new.txt and +two", dir, got.Patch)
		}
	})

	t.Run("empty diff when unchanged", func(t *testing.T) {
		t.Parallel()
		dir := initRepo(t)

		got, err := diffcap.Capture(dir)

		if err != nil {
			t.Fatalf("Capture(%q) error = %v", dir, err)
		}
		if d := cmp.Diff(diffcap.Result{}, got); d != "" {
			t.Errorf("Capture(%q) mismatch (-want +got):\n%s", dir, d)
		}
	})

	t.Run("errors when dir is not a git repo", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir() // no git init: the initial `git add -A` fails

		_, err := diffcap.Capture(dir)

		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("Capture(%q) error = %v, want an *exec.ExitError", dir, err)
		}
	})

	t.Run("propagates a git diff failure", func(t *testing.T) {
		t.Parallel()
		dir := initRepo(t)
		write(t, filepath.Join(dir, "app.txt"), "one\ntwo\n")
		// A broken external diff driver makes `git diff --cached` abort while the
		// preceding `git add -A` still succeeds, exercising Capture's diff error
		// branch. `--numstat` ignores diff.external, so only the diff step fails.
		git(t, dir, "config", "diff.external", "/nonexistent-diff-prog")

		_, err := diffcap.Capture(dir)

		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("Capture(%q) error = %v, want an *exec.ExitError", dir, err)
		}
	})

	// Capture's numstat error branch is unreachable: `git diff --cached --numstat`
	// only fails on repo state that also fails the preceding `git diff --cached`
	// (its work is a subset), so that guard returns first. Error propagation from
	// git is already covered by the not-a-repo and diff-failure cases above.
}

func TestStatsFromPatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		patch string
		want  diffcap.Stats
	}{
		{
			name:  "empty patch",
			patch: "",
			want:  diffcap.Stats{},
		},
		{
			name: "insertion",
			patch: "diff --git a/app.txt b/app.txt\n" +
				"index 5626abf..814f4a4 100644\n" +
				"--- a/app.txt\n" +
				"+++ b/app.txt\n" +
				"@@ -1 +1,2 @@\n" +
				" one\n" +
				"+two\n",
			want: diffcap.Stats{FilesChanged: 1, Insertions: 1, Deletions: 0},
		},
		{
			name: "deletion",
			patch: "diff --git a/app.txt b/app.txt\n" +
				"--- a/app.txt\n" +
				"+++ b/app.txt\n" +
				"@@ -1,2 +1 @@\n" +
				" one\n" +
				"-two\n",
			want: diffcap.Stats{FilesChanged: 1, Insertions: 0, Deletions: 1},
		},
		{
			name: "binary file is not special-cased",
			patch: "diff --git a/img.png b/img.png\n" +
				"new file mode 100644\n" +
				"index 0000000..abc1234\n" +
				"Binary files /dev/null and b/img.png differ\n",
			want: diffcap.Stats{FilesChanged: 1, Insertions: 0, Deletions: 0},
		},
		{
			name: "multiple files",
			patch: "diff --git a/a.txt b/a.txt\n" +
				"--- a/a.txt\n" +
				"+++ b/a.txt\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n" +
				"diff --git a/b.txt b/b.txt\n" +
				"--- a/b.txt\n" +
				"+++ b/b.txt\n" +
				"@@ -0,0 +1 @@\n" +
				"+added\n",
			want: diffcap.Stats{FilesChanged: 2, Insertions: 2, Deletions: 1},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := diffcap.StatsFromPatch(tt.patch)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("StatsFromPatch(%q) mismatch (-want +got):\n%s", tt.patch, d)
			}
		})
	}
}
