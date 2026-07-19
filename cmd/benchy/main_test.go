package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/keyxmare/claude-benchy/internal/docker"
)

// fakeDocker is a docker.Runner that never touches a real daemon: Run reports a
// no-op container (leaving an empty diff, i.e. a degenerate run) and Build
// records its arguments.
type fakeDocker struct {
	runErr        error
	buildErr      error
	builtTag      string
	builtArgs     map[string]string
	builtContext  string
	builtFile     string
	buildCallSeen bool
}

func (f *fakeDocker) Run(_ context.Context, _ docker.RunSpec, _, _ io.Writer) error {
	return f.runErr
}

func (f *fakeDocker) Build(_ context.Context, contextDir, tag, dockerfile string, buildArgs map[string]string) error {
	f.buildCallSeen = true
	f.builtContext = contextDir
	f.builtTag = tag
	f.builtFile = dockerfile
	f.builtArgs = buildArgs
	return f.buildErr
}

func testDeps(dk docker.Runner) (deps, *bytes.Buffer, *bytes.Buffer) {
	var out, errb bytes.Buffer
	d := deps{
		docker: dk,
		stdout: &out,
		stderr: &errb,
		now:    func() time.Time { return time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC) },
	}
	return d, &out, &errb
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeBenchFixture lays down a minimal, valid bench with an app dir and one
// config bundle, and returns the temp root and the bench.yaml path.
func writeBenchFixture(t *testing.T) (dir, benchPath string) {
	t.Helper()
	dir = t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "main.txt"), "hello\n")
	writeFile(t, filepath.Join(dir, "cfg", "CLAUDE.md"), "# cfg\n")
	benchPath = filepath.Join(dir, "bench.yaml")
	writeFile(t, benchPath, `prompt: "do something"
app: ./app
model: sonnet
runs: 1
concurrency: 1
retries: 0
output: ./out
configs:
  - name: baseline
    bundle: ./cfg
`)
	return dir, benchPath
}

func TestRunDispatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		args       []string
		wantErr    string
		wantErrOut string
	}{
		{name: "no subcommand", args: nil, wantErr: "a subcommand is required", wantErrOut: "usage:"},
		{name: "unknown subcommand", args: []string{"bogus"}, wantErr: `unknown subcommand "bogus"`, wantErrOut: "usage:"},
		{name: "help", args: []string{"help"}, wantErr: "", wantErrOut: "usage:"},
		{name: "-h", args: []string{"-h"}, wantErr: "", wantErrOut: "usage:"},
		{name: "--help", args: []string{"--help"}, wantErr: "", wantErrOut: "usage:"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d, _, errb := testDeps(&fakeDocker{})

			err := run(context.Background(), tt.args, d)

			assertErr(t, err, tt.wantErr)
			if !strings.Contains(errb.String(), tt.wantErrOut) {
				t.Errorf("stderr = %q, want it to contain %q", errb.String(), tt.wantErrOut)
			}
		})
	}
}

// TestRunRoutesToSubcommands checks each dispatch arm is reached; the bad args
// make every subcommand fail fast before any real work.
func TestRunRoutesToSubcommands(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
	}{
		{name: "run", args: []string{"run"}},
		{name: "new", args: []string{"new"}},
		{name: "report", args: []string{"report"}},
		{name: "build-image", args: []string{"build-image", "--nope"}},
		{name: "serve", args: []string{"serve", "--nope"}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d, _, _ := testDeps(&fakeDocker{})
			if err := run(context.Background(), tt.args, d); err == nil {
				t.Errorf("run(%v) = nil, want the subcommand's error", tt.args)
			}
		})
	}
}

func TestCmdNew(t *testing.T) {
	t.Parallel()
	t.Run("missing from", func(t *testing.T) {
		t.Parallel()
		d, _, _ := testDeps(&fakeDocker{})
		err := cmdNew(d, nil)
		assertErr(t, err, "usage: benchy new")
	})
	t.Run("flag parse error", func(t *testing.T) {
		t.Parallel()
		d, _, _ := testDeps(&fakeDocker{})
		err := cmdNew(d, []string{"--nope"})
		if err == nil {
			t.Fatalf("cmdNew(--nope) = nil, want a flag error")
		}
	})
	t.Run("import error", func(t *testing.T) {
		t.Parallel()
		d, _, _ := testDeps(&fakeDocker{})
		err := cmdNew(d, []string{"--from", filepath.Join(t.TempDir(), "absent.yaml")})
		if err == nil {
			t.Fatalf("cmdNew(absent) = nil, want an import error")
		}
	})
	t.Run("to stdout", func(t *testing.T) {
		t.Parallel()
		_, bench := writeBenchFixture(t)
		d, out, _ := testDeps(&fakeDocker{})
		if err := cmdNew(d, []string{"--from", bench}); err != nil {
			t.Fatalf("cmdNew = %v, want nil", err)
		}
		if !strings.Contains(out.String(), "app:") {
			t.Errorf("stdout = %q, want the imported bench yaml", out.String())
		}
	})
	t.Run("to output file", func(t *testing.T) {
		t.Parallel()
		dir, bench := writeBenchFixture(t)
		dest := filepath.Join(dir, "new.yaml")
		d, out, _ := testDeps(&fakeDocker{})
		if err := cmdNew(d, []string{"--from", bench, "--output", dest}); err != nil {
			t.Fatalf("cmdNew = %v, want nil", err)
		}
		if _, err := os.Stat(dest); err != nil {
			t.Errorf("output file not written: %v", err)
		}
		if !strings.Contains(out.String(), "imported") {
			t.Errorf("stdout = %q, want an 'imported' line", out.String())
		}
	})
	t.Run("output dir missing", func(t *testing.T) {
		t.Parallel()
		_, bench := writeBenchFixture(t)
		dest := filepath.Join(t.TempDir(), "no-such-dir", "new.yaml")
		d, _, _ := testDeps(&fakeDocker{})
		if err := cmdNew(d, []string{"--from", bench, "--output", dest}); err == nil {
			t.Fatal("cmdNew = nil, want a write error into a missing directory")
		}
	})
}

func TestCmdReport(t *testing.T) {
	t.Parallel()
	t.Run("flag parse error", func(t *testing.T) {
		t.Parallel()
		d, _, _ := testDeps(&fakeDocker{})
		if err := cmdReport(d, []string{"--nope"}); err == nil {
			t.Fatal("cmdReport(--nope) = nil, want a flag error")
		}
	})
	t.Run("wrong arg count", func(t *testing.T) {
		t.Parallel()
		d, _, _ := testDeps(&fakeDocker{})
		assertErr(t, cmdReport(d, nil), "usage: benchy report")
	})
	t.Run("reload error", func(t *testing.T) {
		t.Parallel()
		d, _, _ := testDeps(&fakeDocker{})
		if err := cmdReport(d, []string{filepath.Join(t.TempDir(), "absent")}); err == nil {
			t.Fatal("cmdReport(absent) = nil, want a reload error")
		}
	})
	t.Run("regenerates report", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "bench.json"), `{"prompt":"p","app":"a"}`)
		writeFile(t, filepath.Join(dir, "baseline", "diff.patch"), "diff --git a/f b/f\n+x\n")
		d, out, _ := testDeps(&fakeDocker{})
		if err := cmdReport(d, []string{dir}); err != nil {
			t.Fatalf("cmdReport = %v, want nil", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "report.html")); err != nil {
			t.Errorf("report.html not written: %v", err)
		}
		if !strings.Contains(out.String(), "rapport régénéré") {
			t.Errorf("stdout = %q, want a 'rapport régénéré' line", out.String())
		}
	})
}

func TestCmdBuildImage(t *testing.T) {
	t.Parallel()
	t.Run("flag parse error", func(t *testing.T) {
		t.Parallel()
		d, _, _ := testDeps(&fakeDocker{})
		if err := cmdBuildImage(context.Background(), d, []string{"--nope"}); err == nil {
			t.Fatal("cmdBuildImage(--nope) = nil, want a flag error")
		}
	})
	t.Run("without claude-version", func(t *testing.T) {
		t.Parallel()
		fd := &fakeDocker{}
		d, out, _ := testDeps(fd)
		if err := cmdBuildImage(context.Background(), d, []string{"--tag", "t:1"}); err != nil {
			t.Fatalf("cmdBuildImage = %v, want nil", err)
		}
		if len(fd.builtArgs) != 0 {
			t.Errorf("buildArgs = %v, want empty", fd.builtArgs)
		}
		if fd.builtTag != "t:1" {
			t.Errorf("tag = %q, want %q", fd.builtTag, "t:1")
		}
		if !strings.Contains(out.String(), "built image t:1") {
			t.Errorf("stdout = %q, want a 'built image' line", out.String())
		}
	})
	t.Run("with claude-version", func(t *testing.T) {
		t.Parallel()
		fd := &fakeDocker{}
		d, _, _ := testDeps(fd)
		if err := cmdBuildImage(context.Background(), d, []string{"--claude-version", "1.2.3"}); err != nil {
			t.Fatalf("cmdBuildImage = %v, want nil", err)
		}
		if got := fd.builtArgs["CLAUDE_VERSION"]; got != "1.2.3" {
			t.Errorf("CLAUDE_VERSION = %q, want %q", got, "1.2.3")
		}
	})
	t.Run("build error", func(t *testing.T) {
		t.Parallel()
		fd := &fakeDocker{buildErr: io.ErrUnexpectedEOF}
		d, _, _ := testDeps(fd)
		if err := cmdBuildImage(context.Background(), d, nil); err == nil {
			t.Fatal("cmdBuildImage = nil, want the build error")
		}
	})
}

func TestCmdRun(t *testing.T) {
	t.Parallel()
	t.Run("flag parse error", func(t *testing.T) {
		t.Parallel()
		d, _, _ := testDeps(&fakeDocker{})
		if err := cmdRun(context.Background(), d, []string{"--nope"}); err == nil {
			t.Fatal("cmdRun(--nope) = nil, want a flag error")
		}
	})
	t.Run("wrong arg count", func(t *testing.T) {
		t.Parallel()
		d, _, _ := testDeps(&fakeDocker{})
		assertErr(t, cmdRun(context.Background(), d, nil), "usage: benchy run")
	})
	t.Run("spec load error", func(t *testing.T) {
		t.Parallel()
		d, _, _ := testDeps(&fakeDocker{})
		if err := cmdRun(context.Background(), d, []string{filepath.Join(t.TempDir(), "absent.yaml")}); err == nil {
			t.Fatal("cmdRun(absent) = nil, want a spec load error")
		}
	})
	t.Run("output path unusable", func(t *testing.T) {
		t.Parallel()
		dir, bench := writeBenchFixture(t)
		// A file at ./out makes runner.Run's MkdirAll(out/<timestamp>) fail.
		writeFile(t, filepath.Join(dir, "out"), "not a directory")
		d, _, _ := testDeps(&fakeDocker{})
		if err := cmdRun(context.Background(), d, []string{bench}); err == nil {
			t.Fatal("cmdRun = nil, want a runner error when the output path is a file")
		}
	})
	t.Run("runs and writes report", func(t *testing.T) {
		t.Parallel()
		dir, bench := writeBenchFixture(t)
		d, out, _ := testDeps(&fakeDocker{})

		if err := cmdRun(context.Background(), d, []string{bench}); err != nil {
			t.Fatalf("cmdRun = %v, want nil", err)
		}

		if !strings.Contains(out.String(), "done:") || !strings.Contains(out.String(), "report:") {
			t.Errorf("stdout = %q, want 'done:' and 'report:' lines", out.String())
		}
		reportPath := filepath.Join(dir, "out", "20240102-030405", "report.html")
		if _, err := os.Stat(reportPath); err != nil {
			t.Errorf("report not written at %s: %v", reportPath, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "out", "20240102-030405", "bench.yaml")); err != nil {
			t.Errorf("source bench not persisted: %v", err)
		}
	})
}

func TestResolveImage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		specImage string
		want      string
	}{
		{name: "explicit flag wins", args: []string{"--image", "flag:img"}, specImage: "spec:img", want: "flag:img"},
		{name: "spec image when no flag", args: nil, specImage: "spec:img", want: "spec:img"},
		{name: "default when neither", args: nil, specImage: "", want: defaultImage},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := flag.NewFlagSet("run", flag.ContinueOnError)
			image := fs.String("image", defaultImage, "")
			if err := fs.Parse(tt.args); err != nil {
				t.Fatal(err)
			}

			got := resolveImage(fs, *image, tt.specImage)

			if got != tt.want {
				t.Errorf("resolveImage(%v, %q) = %q, want %q", tt.args, tt.specImage, got, tt.want)
			}
		})
	}
}

func TestCmdServe(t *testing.T) {
	t.Parallel()
	t.Run("flag parse error", func(t *testing.T) {
		t.Parallel()
		d, _, _ := testDeps(&fakeDocker{})
		if err := cmdServe(context.Background(), d, []string{"--nope"}); err == nil {
			t.Fatal("cmdServe(--nope) = nil, want a flag error")
		}
	})
	t.Run("shuts down on context cancel", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		d, _, _ := testDeps(&fakeDocker{})
		if err := cmdServe(ctx, d, []string{"--addr", "127.0.0.1:0", "--root", t.TempDir()}); err != nil {
			t.Fatalf("cmdServe = %v, want nil after shutdown", err)
		}
	})
	t.Run("listen error", func(t *testing.T) {
		t.Parallel()
		d, _, _ := testDeps(&fakeDocker{})
		err := cmdServe(context.Background(), d, []string{"--addr", "127.0.0.1:-1", "--root", t.TempDir()})
		if err == nil {
			t.Fatal("cmdServe(bad addr) = nil, want a listen error")
		}
	})
}

// Lines left uncovered on purpose, each unreachable deterministically here:
//   - main: a thin wiring shell (signal setup, os.Exit) that cannot run under
//     the test binary; all its logic lives in run and the cmd* functions.
//   - the report.Write error returns in cmdRun/cmdReport: report.Write fails
//     only on an I/O error writing into the just-created output directory, which
//     the tests (running as root in the container) cannot provoke.
//   - the "source bench unreadable" skip in cmdRun: the bench was just read by
//     spec.Load, so its re-read never fails during a test run.
//   - the filepath.Abs / server.New error returns: filepath.Abs fails only when
//     the working directory is unavailable, which is not forceable.

func assertErr(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %v, want it to contain %q", err, want)
	}
}
