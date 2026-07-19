package docker

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRunArgs(t *testing.T) {
	got := RunArgs(RunSpec{
		Image:     "img",
		WorkDir:   "/host/work",
		CredsFile: "/host/.credentials.json",
		Env:       map[string]string{"B": "2", "A": "1"},
		Args:      []string{"-p", "hello", "--model", "sonnet"},
	})
	want := []string{
		"run", "--rm",
		"-v", "/host/work:/work",
		"-w", "/work",
		"-v", "/host/.credentials.json:/benchy/creds/.credentials.json:ro",
		"-e", "A=1",
		"-e", "B=2",
		"img",
		"-p", "hello", "--model", "sonnet",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RunArgs() mismatch (-want +got):\n%s", diff)
	}
}

func TestRunArgsWithoutCreds(t *testing.T) {
	got := RunArgs(RunSpec{Image: "img", WorkDir: "/w", Args: []string{"-p", "x"}})
	want := []string{"run", "--rm", "-v", "/w:/work", "-w", "/work", "img", "-p", "x"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RunArgs() mismatch (-want +got):\n%s", diff)
	}
}

func TestTailWriterKeepsLastBytesFlattened(t *testing.T) {
	var w tailWriter
	if _, err := w.Write([]byte("  Unable to find image\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := w.Write([]byte("pull access denied  \n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := w.oneLine()
	want := "Unable to find image pull access denied"
	if got != want {
		t.Fatalf("oneLine mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestTailWriterCapsRetainedBytes(t *testing.T) {
	var w tailWriter
	if _, err := w.Write([]byte(strings.Repeat("a", maxTail+100))); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := len(w.oneLine()); got != maxTail {
		t.Fatalf("retained %d bytes, want %d", got, maxTail)
	}
}

func TestTailWriterEmpty(t *testing.T) {
	var w tailWriter
	if got := w.oneLine(); got != "" {
		t.Fatalf("oneLine = %q, want empty", got)
	}
}

func TestRunArgsWithEntrypoint(t *testing.T) {
	got := RunArgs(RunSpec{
		Image:      "img",
		WorkDir:    "/w",
		Entrypoint: "sh",
		Args:       []string{"-c", "go test ./..."},
	})
	want := []string{
		"run", "--rm", "-v", "/w:/work", "-w", "/work",
		"--entrypoint", "sh", "img", "-c", "go test ./...",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RunArgs() mismatch (-want +got):\n%s", diff)
	}
}
