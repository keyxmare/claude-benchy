// Command benchy runs a benchmark comparing several Claude Code configurations
// against the same prompt and application under test.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/keyxmare/claude-benchy/internal/docker"
	"github.com/keyxmare/claude-benchy/internal/report"
	"github.com/keyxmare/claude-benchy/internal/runner"
	"github.com/keyxmare/claude-benchy/internal/spec"
)

const (
	defaultImage      = "claude-benchy:latest"
	dockerContextDir  = "build/docker"
	timestampLayout   = "20060102-150405"
	generatedAtLayout = "2006-01-02 15:04:05 MST"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("a subcommand is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch args[0] {
	case "run":
		return cmdRun(ctx, args[1:])
	case "build-image":
		return cmdBuildImage(ctx, args[1:])
	case "-h", "--help", "help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func cmdRun(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
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

	now := time.Now()
	outputRoot := filepath.Join(s.Output, now.Format(timestampLayout))

	rep, err := runner.Run(ctx, s, outputRoot, now.Format(generatedAtLayout), runner.Options{
		Image:  *image,
		Docker: docker.NewCLI(),
	})
	if err != nil {
		return err
	}

	if err := writeReports(outputRoot, rep); err != nil {
		return err
	}

	fmt.Printf("done: %d run(s) → %s\n", len(rep.Runs), outputRoot)
	fmt.Printf("report: %s\n", filepath.Join(outputRoot, "report.html"))
	return nil
}

func cmdBuildImage(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("build-image", flag.ContinueOnError)
	tag := fs.String("tag", defaultImage, "image tag to build")
	contextDir := fs.String("context", dockerContextDir, "docker build context")
	claudeVersion := fs.String("claude-version", "", "pin the claude CLI version")
	if err := fs.Parse(args); err != nil {
		return err
	}

	buildArgs := map[string]string{}
	if *claudeVersion != "" {
		buildArgs["CLAUDE_VERSION"] = *claudeVersion
	}
	if err := docker.NewCLI().Build(ctx, *contextDir, *tag, buildArgs); err != nil {
		return err
	}
	fmt.Printf("built image %s\n", *tag)
	return nil
}

func writeReports(outputRoot string, rep report.Report) error {
	md, err := os.Create(filepath.Join(outputRoot, "report.md"))
	if err != nil {
		return err
	}
	defer md.Close()
	if err := report.WriteMarkdown(md, rep); err != nil {
		return err
	}

	html, err := os.Create(filepath.Join(outputRoot, "report.html"))
	if err != nil {
		return err
	}
	defer html.Close()
	return report.WriteHTML(html, rep)
}

func usage() {
	fmt.Fprint(os.Stderr, `benchy — benchmark Claude Code configurations

usage:
  benchy run [--image IMG] <bench.yaml>     run a benchmark
  benchy build-image [--tag T] [--context D] [--claude-version V]
                                            build the sandbox image

`)
}
