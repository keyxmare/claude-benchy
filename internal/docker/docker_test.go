package docker

import (
	"slices"
	"testing"
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
	if !slices.Equal(got, want) {
		t.Fatalf("RunArgs mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestRunArgsWithoutCreds(t *testing.T) {
	got := RunArgs(RunSpec{Image: "img", WorkDir: "/w", Args: []string{"-p", "x"}})
	want := []string{"run", "--rm", "-v", "/w:/work", "-w", "/work", "img", "-p", "x"}
	if !slices.Equal(got, want) {
		t.Fatalf("RunArgs mismatch\n got: %v\nwant: %v", got, want)
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
	if !slices.Equal(got, want) {
		t.Fatalf("RunArgs mismatch\n got: %v\nwant: %v", got, want)
	}
}
