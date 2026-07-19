// Package docker runs the Claude sandbox container by shelling out to the
// docker CLI. The command construction is kept as a pure function so it can be
// unit tested without a Docker daemon.
package docker

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Container paths used by the sandbox image.
const (
	workMount  = "/work"
	credsMount = "/benchy/creds/.credentials.json"
)

// RunSpec fully describes one sandbox invocation.
type RunSpec struct {
	Image      string
	WorkDir    string            // host path mounted read-write at /work
	CredsFile  string            // host credentials file mounted read-only
	Env        map[string]string // extra environment for the container
	Entrypoint string            // override the image entrypoint (empty: run claude)
	Args       []string          // arguments passed to the entrypoint (or to claude)
}

// Runner executes and builds sandbox containers.
type Runner interface {
	Run(ctx context.Context, spec RunSpec, stdout, stderr io.Writer) error
	Build(ctx context.Context, contextDir, tag, dockerfile string, buildArgs map[string]string) error
}

// CLI is a Runner backed by the real docker binary.
type CLI struct {
	Bin string
}

// NewCLI returns a CLI runner using the docker binary on PATH.
func NewCLI() *CLI {
	return &CLI{Bin: "docker"}
}

// Run executes the sandbox container and streams its output. On failure the
// error carries the tail of the container's stderr — for a daemon-level failure
// (missing image, bad mount) this is the only diagnostic, so surfacing it turns
// an opaque "exit status 125" into an actionable message.
func (c *CLI) Run(ctx context.Context, spec RunSpec, stdout, stderr io.Writer) error {
	var tail tailWriter
	cmd := exec.CommandContext(ctx, c.Bin, RunArgs(spec)...)
	cmd.Stdout = stdout
	if stderr != nil {
		cmd.Stderr = io.MultiWriter(stderr, &tail)
	} else {
		cmd.Stderr = &tail
	}
	if err := cmd.Run(); err != nil {
		if msg := tail.oneLine(); msg != "" {
			return fmt.Errorf("docker run: %w: %s", err, msg)
		}
		return fmt.Errorf("docker run: %w", err)
	}
	return nil
}

// tailWriter keeps only the last maxTail bytes written to it, so a verbose
// stream leaves a bounded, still-useful excerpt for error messages.
type tailWriter struct {
	buf []byte
}

const maxTail = 512

func (w *tailWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	if len(w.buf) > maxTail {
		w.buf = w.buf[len(w.buf)-maxTail:]
	}
	return len(p), nil
}

// oneLine returns the retained tail trimmed and flattened to a single line so
// it stays legible when folded into an error or a progress log.
func (w *tailWriter) oneLine() string {
	s := strings.TrimSpace(string(w.buf))
	return strings.Join(strings.Fields(s), " ")
}

// Build builds the sandbox image from contextDir. When dockerfile is non-empty
// it selects a specific Dockerfile within the context.
func (c *CLI) Build(ctx context.Context, contextDir, tag, dockerfile string, buildArgs map[string]string) error {
	args := []string{"build", "-t", tag}
	if dockerfile != "" {
		args = append(args, "-f", filepath.Join(contextDir, dockerfile))
	}
	for _, k := range sortedKeys(buildArgs) {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, buildArgs[k]))
	}
	args = append(args, contextDir)
	cmd := exec.CommandContext(ctx, c.Bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker build: %w: %s", err, out)
	}
	return nil
}

// RunArgs builds the docker CLI arguments for a sandbox run.
func RunArgs(spec RunSpec) []string {
	args := []string{
		"run", "--rm",
		"-v", spec.WorkDir + ":" + workMount,
		"-w", workMount,
	}
	if spec.CredsFile != "" {
		args = append(args, "-v", spec.CredsFile+":"+credsMount+":ro")
	}
	for _, k := range sortedKeys(spec.Env) {
		args = append(args, "-e", k+"="+spec.Env[k])
	}
	if spec.Entrypoint != "" {
		args = append(args, "--entrypoint", spec.Entrypoint)
	}
	args = append(args, spec.Image)
	args = append(args, spec.Args...)
	return args
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
