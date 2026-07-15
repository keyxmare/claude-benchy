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
	"strings"
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
	// Log, when set, receives human-readable progress lines as jobs start and
	// finish. It may be called concurrently from several goroutines.
	Log func(string)
}

func (o Options) log(format string, args ...any) {
	if o.Log != nil {
		o.Log(fmt.Sprintf(format, args...))
	}
}

type job struct {
	config spec.Config
	run    int
	index  int
}

type runMeta struct {
	Config string `json:"config"`
	Run    int    `json:"run"`
	Model  string `json:"model"`
}

// Run executes every job described by s, writing artifacts under outputRoot,
// and returns the aggregated report. Individual run failures are recorded in
// the report rather than aborting the whole benchmark.
func Run(ctx context.Context, s *spec.Spec, outputRoot, generatedAt string, opts Options) (report.Report, error) {
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return report.Report{}, err
	}

	writeJSON(filepath.Join(outputRoot, "bench.json"), benchMeta{Prompt: commonPrompt(s), App: s.App})

	jobs := expand(s)
	results := make([]report.RunReport, len(jobs))
	opts.log("lancement de %d run(s) sur %d config(s), concurrence %d", len(jobs), len(s.Configs), s.Concurrency)

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
	_ = os.MkdirAll(artifactDir, 0o755)
	writeJSON(filepath.Join(artifactDir, "meta.json"), runMeta{Config: j.config.Name, Run: j.run, Model: j.config.Model})
	opts.log("▶ %-18s démarrage (%s)", rel, j.config.Model)

	if err := workspace.Prepare(s.App, j.config.Bundle, workspaceDir); err != nil {
		res.Err = err.Error()
		opts.log("✗ %-18s échec préparation: %s", rel, err)
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
		res.Patch = diff.Patch
		_ = os.WriteFile(filepath.Join(artifactDir, "diff.patch"), []byte(diff.Patch), 0o644)
	}

	if res.Err != "" {
		opts.log("✗ %-18s erreur: %s", rel, firstLine(res.Err))
	} else {
		opts.log("✓ %-18s %s · $%.4f · %d fichier(s) (+%d/-%d)", rel,
			seconds(res.Metrics.DurationMS), res.Metrics.TotalCostUSD,
			res.Diff.FilesChanged, res.Diff.Insertions, res.Diff.Deletions)
	}
	return res
}

func seconds(ms int) string {
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
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
