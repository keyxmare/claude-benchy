package diffcap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	return dir
}

func TestCaptureChanges(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "app.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Capture(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stats.FilesChanged != 2 {
		t.Errorf("FilesChanged = %d, want 2", res.Stats.FilesChanged)
	}
	if res.Stats.Insertions != 2 {
		t.Errorf("Insertions = %d, want 2", res.Stats.Insertions)
	}
	if !strings.Contains(res.Patch, "new.txt") || !strings.Contains(res.Patch, "+two") {
		t.Errorf("unexpected patch:\n%s", res.Patch)
	}
}

func TestCaptureNoChanges(t *testing.T) {
	dir := initRepo(t)
	res, err := Capture(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stats.FilesChanged != 0 || res.Patch != "" {
		t.Errorf("expected empty diff, got %+v / %q", res.Stats, res.Patch)
	}
}
