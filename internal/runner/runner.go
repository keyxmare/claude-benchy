// Package runner orchestrates a benchmark: for every configuration (repeated
// runs times) it prepares an isolated workspace, runs Claude in a sandbox
// container, then captures the transcript, metrics and diff.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/diffcap"
	"github.com/keyxmare/claude-benchy/internal/docker"
	"github.com/keyxmare/claude-benchy/internal/report"
	"github.com/keyxmare/claude-benchy/internal/spec"
	"github.com/keyxmare/claude-benchy/internal/workspace"
)

// Options configures a benchmark run.
type Options struct {
	Image  string
	Docker docker.Runner
}

type job struct {
	config spec.Config
	run    int
	index  int
}

// Run executes every job described by s, writing artifacts under outputRoot,
// and returns the aggregated report. Individual run failures are recorded in
// the report rather than aborting the whole benchmark.
func Run(ctx context.Context, s *spec.Spec, outputRoot, generatedAt string, opts Options) (report.Report, error) {
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return report.Report{}, err
	}

	jobs := expand(s)
	results := make([]report.RunReport, len(jobs))

	sem := make(chan struct{}, s.Concurrency)
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			results[j.index] = execJob(ctx, s, j, outputRoot, opts)
		}(j)
	}
	wg.Wait()

	return report.Report{
		Prompt:      commonPrompt(s),
		App:         s.App,
		GeneratedAt: generatedAt,
		Runs:        results,
	}, nil
}

func execJob(ctx context.Context, s *spec.Spec, j job, outputRoot string, opts Options) report.RunReport {
	rel := j.config.Name
	if s.Runs > 1 {
		rel = filepath.Join(j.config.Name, fmt.Sprintf("run-%d", j.run))
	}
	artifactDir := filepath.Join(outputRoot, rel)
	workspaceDir := filepath.Join(artifactDir, "workspace")

	res := report.RunReport{
		Config:      j.config.Name,
		Run:         j.run,
		Model:       j.config.Model,
		ArtifactDir: rel,
	}

	if err := workspace.Prepare(s.App, j.config.Bundle, workspaceDir); err != nil {
		res.Err = err.Error()
		return res
	}

	metrics, err := runClaude(ctx, s, j, artifactDir, workspaceDir, opts)
	if err != nil {
		res.Err = err.Error()
	}
	res.Metrics = metrics

	if diff, err := diffcap.Capture(workspaceDir); err != nil {
		if res.Err == "" {
			res.Err = err.Error()
		}
	} else {
		res.Diff = diff.Stats
		_ = os.WriteFile(filepath.Join(artifactDir, "diff.patch"), []byte(diff.Patch), 0o644)
	}
	return res
}

func runClaude(ctx context.Context, s *spec.Spec, j job, artifactDir, workspaceDir string, opts Options) (claude.Metrics, error) {
	transcript, err := os.Create(filepath.Join(artifactDir, "transcript.jsonl"))
	if err != nil {
		return claude.Metrics{}, err
	}
	defer transcript.Close()

	logFile, err := os.Create(filepath.Join(artifactDir, "stdout.log"))
	if err != nil {
		return claude.Metrics{}, err
	}
	defer logFile.Close()

	runSpec := docker.RunSpec{
		Image:     opts.Image,
		WorkDir:   workspaceDir,
		CredsFile: s.CredsFile,
		Env:       map[string]string{"DISABLE_AUTOUPDATER": "1"},
		Args:      claudeArgs(j.config),
	}
	runErr := opts.Docker.Run(ctx, runSpec, transcript, logFile)

	if _, err := transcript.Seek(0, 0); err != nil {
		return claude.Metrics{}, err
	}
	metrics, parseErr := claude.Parse(transcript)
	if parseErr != nil {
		if runErr != nil {
			return metrics, runErr
		}
		return metrics, parseErr
	}
	writeJSON(filepath.Join(artifactDir, "result.json"), metrics)
	return metrics, runErr
}

func claudeArgs(c spec.Config) []string {
	return []string{
		"-p", c.Prompt,
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"--setting-sources", "project,local",
		"--model", c.Model,
	}
}

func expand(s *spec.Spec) []job {
	var jobs []job
	idx := 0
	for _, c := range s.Configs {
		for run := 1; run <= s.Runs; run++ {
			jobs = append(jobs, job{config: c, run: run, index: idx})
			idx++
		}
	}
	return jobs
}

func commonPrompt(s *spec.Spec) string {
	if len(s.Configs) == 0 {
		return ""
	}
	first := s.Configs[0].Prompt
	for _, c := range s.Configs[1:] {
		if c.Prompt != first {
			return "(varies per config)"
		}
	}
	return first
}

func writeJSON(path string, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return
	}
	_ = os.WriteFile(path, buf.Bytes(), 0o644)
}
