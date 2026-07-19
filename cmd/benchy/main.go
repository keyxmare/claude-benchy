// Command benchy runs a benchmark comparing several Claude Code configurations
// against the same prompt and application under test.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/keyxmare/claude-benchy/internal/docker"
	"github.com/keyxmare/claude-benchy/internal/report"
	"github.com/keyxmare/claude-benchy/internal/runner"
	"github.com/keyxmare/claude-benchy/internal/server"
	"github.com/keyxmare/claude-benchy/internal/spec"
)

const defaultAddr = "127.0.0.1:80"

const (
	defaultImage      = "claude-benchy:latest"
	dockerContextDir  = "build/docker"
	timestampLayout   = "20060102-150405"
	generatedAtLayout = "2006-01-02 15:04:05 MST"
)

// deps holds the process's injectable collaborators so every command can be
// driven in tests without a real Docker daemon, the wall clock or the process
// stdio. main wires the real ones; tests wire fakes.
type deps struct {
	docker docker.Runner
	stdout io.Writer
	stderr io.Writer
	now    func() time.Time
}

// main wires the real collaborators and turns a command error into an exit
// code. The testable logic lives in run and the cmd* functions; main itself is
// a thin, unit-untestable shell (signal wiring, os.Exit).
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	d := deps{docker: docker.NewCLI(), stdout: os.Stdout, stderr: os.Stderr, now: time.Now}
	err := run(ctx, os.Args[1:], d)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, d deps) error {
	if len(args) == 0 {
		usage(d.stderr)
		return fmt.Errorf("a subcommand is required")
	}

	switch args[0] {
	case "run":
		return cmdRun(ctx, d, args[1:])
	case "new":
		return cmdNew(d, args[1:])
	case "report":
		return cmdReport(d, args[1:])
	case "build-image":
		return cmdBuildImage(ctx, d, args[1:])
	case "serve":
		return cmdServe(ctx, d, args[1:])
	case "-h", "--help", "help":
		usage(d.stderr)
		return nil
	default:
		usage(d.stderr)
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func cmdRun(ctx context.Context, d deps, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(d.stderr)
	image := fs.String("image", defaultImage, "sandbox image to run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: benchy run [--image IMG] <bench.yaml>")
	}

	s, err := spec.Load(fs.Arg(0))
	if err != nil {
		return err
	}

	now := d.now()
	outputRoot := filepath.Join(s.Output, now.Format(timestampLayout))

	rep, err := runner.Run(ctx, s, outputRoot, now.Format(generatedAtLayout), runner.Options{
		Image:  resolveImage(fs, *image, s.Sandbox.Image),
		Docker: d.docker,
		Log:    func(line string) { _, _ = fmt.Fprintln(d.stderr, line) },
	})
	if err != nil {
		return err
	}

	// Persist the source bench next to the results so its configuration can be
	// reused (e.g. from the web dashboard) without re-typing it.
	if raw, readErr := os.ReadFile(fs.Arg(0)); readErr == nil {
		_ = os.WriteFile(filepath.Join(outputRoot, "bench.yaml"), raw, 0o644)
	}

	if err := report.Write(outputRoot, rep); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(d.stdout, "done: %d run(s) → %s\n", len(rep.Runs), outputRoot)
	_, _ = fmt.Fprintf(d.stdout, "report: %s\n", filepath.Join(outputRoot, "report.html"))
	return nil
}

// resolveImage picks the sandbox image: an explicit --image flag wins, then the
// spec's sandbox.image, then the built-in default.
func resolveImage(fs *flag.FlagSet, flagValue, specImage string) string {
	explicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "image" {
			explicit = true
		}
	})
	if explicit {
		return flagValue
	}
	if specImage != "" {
		return specImage
	}
	return flagValue
}

// cmdNew imports an existing bench.yaml as the starting point for a new one,
// rewriting its relative paths to absolute against the source file's directory
// so the result is runnable from anywhere. It writes to --output or stdout.
func cmdNew(d deps, args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(d.stderr)
	from := fs.String("from", "", "existing bench.yaml to import")
	out := fs.String("output", "", "write the imported bench.yaml here (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *from == "" {
		return fmt.Errorf("usage: benchy new --from <bench.yaml> [--output <dest.yaml>]")
	}

	src, err := filepath.Abs(*from)
	if err != nil {
		return err // filepath.Abs only fails when the working directory is unavailable.
	}
	raw, err := spec.Import(src)
	if err != nil {
		return err
	}
	if *out == "" {
		_, err := d.stdout.Write(raw)
		return err
	}
	if err := os.WriteFile(*out, raw, 0o644); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(d.stdout, "imported %s → %s\n", *from, *out)
	return nil
}

func cmdReport(d deps, args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(d.stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: benchy report <dossier-de-résultats>")
	}
	dir := fs.Arg(0)

	prompt, app := runner.BenchInfo(dir)
	rep, err := runner.Reload(dir, app, prompt, d.now().Format(generatedAtLayout))
	if err != nil {
		return err
	}
	if err := report.Write(dir, rep); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(d.stdout, "rapport régénéré: %s\n", filepath.Join(dir, "report.html"))
	return nil
}

func cmdBuildImage(ctx context.Context, d deps, args []string) error {
	fs := flag.NewFlagSet("build-image", flag.ContinueOnError)
	fs.SetOutput(d.stderr)
	tag := fs.String("tag", defaultImage, "image tag to build")
	contextDir := fs.String("context", dockerContextDir, "docker build context")
	dockerfile := fs.String("dockerfile", "", "Dockerfile to use (relative to context; default: Dockerfile)")
	claudeVersion := fs.String("claude-version", "", "pin the claude CLI version")
	if err := fs.Parse(args); err != nil {
		return err
	}

	buildArgs := map[string]string{}
	if *claudeVersion != "" {
		buildArgs["CLAUDE_VERSION"] = *claudeVersion
	}
	if err := d.docker.Build(ctx, *contextDir, *tag, *dockerfile, buildArgs); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(d.stdout, "built image %s\n", *tag)
	return nil
}

func cmdServe(ctx context.Context, d deps, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(d.stderr)
	addr := fs.String("addr", defaultAddr, "listen address (localhost only by default: the server can launch Docker with mounted OAuth creds)")
	root := fs.String("root", ".", "directory scanned for past benchmarks and base for the form's relative paths")
	image := fs.String("image", defaultImage, "default sandbox image")
	if err := fs.Parse(args); err != nil {
		return err
	}

	srv, err := server.New(*root, *image, d.docker)
	if err != nil {
		return err // server.New only fails when the working directory is unavailable.
	}

	httpSrv := &http.Server{Addr: *addr, Handler: srv.Handler()}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdown)
	}()

	_, _ = fmt.Fprintf(d.stdout, "benchy serve → http://%s (racine : %s)\n", *addr, *root)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `benchy — benchmark Claude Code configurations

usage:
  benchy run [--image IMG] <bench.yaml>     run a benchmark
  benchy new --from <bench.yaml> [--output F]
                                            import a bench.yaml (paths made absolute) as a new starting point
  benchy report <results-dir>               re-render report from artifacts
  benchy serve [--addr A] [--root D] [--image IMG]
                                            web dashboard: configure, launch and browse benchmarks
  benchy build-image [--tag T] [--context D] [--claude-version V]
                                            build the sandbox image

`)
}
