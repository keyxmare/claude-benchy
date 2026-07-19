package docker_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keyxmare/claude-benchy/internal/docker"
)

func TestNewCLI(t *testing.T) {
	t.Parallel()

	got := docker.NewCLI()

	if got.Bin != "docker" {
		t.Errorf("NewCLI().Bin = %q, want %q", got.Bin, "docker")
	}
}

func TestCLIRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		bin        func(t *testing.T) string
		withStderr bool
		wantErr    bool
		wantTail   string
	}{
		{
			name:       "success streams without error",
			bin:        func(*testing.T) string { return "true" },
			withStderr: true,
			wantErr:    false,
		},
		{
			name:       "failure without stderr writer and empty tail",
			bin:        func(*testing.T) string { return "false" },
			withStderr: false,
			wantErr:    true,
		},
		{
			name:       "failure surfaces stderr tail",
			bin:        boomScript,
			withStderr: true,
			wantErr:    true,
			wantTail:   "boom",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &docker.CLI{Bin: tt.bin(t)}
			spec := docker.RunSpec{Image: "img", WorkDir: t.TempDir()}
			var stdout, stderr bytes.Buffer
			var stderrW io.Writer
			if tt.withStderr {
				stderrW = &stderr
			}

			err := c.Run(context.Background(), spec, &stdout, stderrW)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				return
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Errorf("Run() error = %v, want it to wrap *exec.ExitError", err)
			}
			if tt.wantTail != "" && !strings.Contains(err.Error(), tt.wantTail) {
				t.Errorf("Run() error = %v, want tail %q surfaced", err, tt.wantTail)
			}
		})
	}
}

func TestCLIBuildArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		dockerfile string
		buildArgs  map[string]string
		wantArgs   func(ctx string) []string
	}{
		{
			name:       "no dockerfile no build args",
			dockerfile: "",
			buildArgs:  nil,
			wantArgs: func(ctx string) []string {
				return []string{"build", "-t", "tag", ctx}
			},
		},
		{
			name:       "dockerfile and build args sorted deterministically",
			dockerfile: "Dockerfile.go",
			buildArgs:  map[string]string{"B": "2", "A": "1"},
			wantArgs: func(ctx string) []string {
				return []string{
					"build", "-t", "tag",
					"-f", filepath.Join(ctx, "Dockerfile.go"),
					"--build-arg", "A=1",
					"--build-arg", "B=2",
					ctx,
				}
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bin, argsFile := recordingScript(t)
			c := &docker.CLI{Bin: bin}
			ctxDir := t.TempDir()

			err := c.Build(context.Background(), ctxDir, "tag", tt.dockerfile, tt.buildArgs)

			if err != nil {
				t.Fatalf("Build() error = %v, want nil", err)
			}
			got := recordedArgs(t, argsFile)
			if diff := cmp.Diff(tt.wantArgs(ctxDir), got); diff != "" {
				t.Errorf("Build() args mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCLIBuildError(t *testing.T) {
	t.Parallel()

	c := &docker.CLI{Bin: "false"}

	err := c.Build(context.Background(), t.TempDir(), "tag", "", nil)

	if err == nil {
		t.Fatalf("Build() error = nil, want non-nil")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("Build() error = %v, want it to wrap *exec.ExitError", err)
	}
}

func boomScript(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "boom.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho boom >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func recordingScript(t *testing.T) (bin, argsFile string) {
	t.Helper()

	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args.txt")
	bin = filepath.Join(dir, "record.sh")
	content := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\n"
	if err := os.WriteFile(bin, []byte(content), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return bin, argsFile
}

func recordedArgs(t *testing.T, argsFile string) []string {
	t.Helper()

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read recorded args: %v", err)
	}
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
