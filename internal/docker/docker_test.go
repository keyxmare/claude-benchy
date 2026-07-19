package docker_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keyxmare/claude-benchy/internal/docker"
)

func TestRunArgs(t *testing.T) {
	got := docker.RunArgs(docker.RunSpec{
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
	got := docker.RunArgs(docker.RunSpec{Image: "img", WorkDir: "/w", Args: []string{"-p", "x"}})
	want := []string{"run", "--rm", "-v", "/w:/work", "-w", "/work", "img", "-p", "x"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RunArgs() mismatch (-want +got):\n%s", diff)
	}
}

func TestRunArgsWithEntrypoint(t *testing.T) {
	got := docker.RunArgs(docker.RunSpec{
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
