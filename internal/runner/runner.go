// Package runner orchestrates a benchmark: for every configuration (repeated
// runs times) it prepares an isolated workspace, runs Claude in a sandbox
// container, then captures the transcript, metrics and diff.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	// Agent, when set, receives per-agent updates: output lines rendered live
	// from each run's transcript, and lifecycle status changes. It may be called
	// concurrently from several goroutines.
	Agent func(AgentEvent)
}

// AgentEvent is a live update about one agent — a single config's run. Line
// carries a rendered transcript line; Status carries a lifecycle change
// ("running", "done", "failed", "degenerate"). Either may be empty.
type AgentEvent struct {
	Agent  string `json:"agent"`
	Line   string `json:"line,omitempty"`
	Status string `json:"status,omitempty"`
}

func (o Options) log(format string, args ...any) {
	if o.Log != nil {
		o.Log(fmt.Sprintf(format, args...))
	}
}

func (o Options) agent(ev AgentEvent) {
	if o.Agent != nil {
		o.Agent(ev)
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

	rep := report.Report{
		Prompt:      commonPrompt(s),
		App:         s.App,
		GeneratedAt: generatedAt,
		Runs:        results,
	}

	if s.Evaluate.Enabled() {
		switch {
		case len(s.Evaluate.Rubric) == 0:
			opts.log("• évaluation : checks uniquement (pas de rubric à juger)")
		case !anyOK(results):
			opts.log("• évaluation ignorée : aucun run réussi à juger")
		default:
			opts.log("▶ évaluation de l'attendu (juge : %s)", s.Evaluate.Model)
			if eval, err := judge(ctx, s, rep, outputRoot, opts); err != nil {
				opts.log("✗ évaluation: %s", firstLine(err.Error()))
			} else {
				rep.Evaluation = eval
				writeJSON(filepath.Join(outputRoot, "evaluation.json"), eval)
				opts.log("✓ évaluation terminée")
			}
		}
	}

	return rep, nil
}

func execJob(ctx context.Context, s *spec.Spec, j job, outputRoot string, opts Options) report.RunReport {
	rel := j.config.Name
	if s.Runs > 1 {
		rel = filepath.Join(j.config.Name, fmt.Sprintf("run-%d", j.run))
	}
	artifactDir := filepath.Join(outputRoot, rel)
	workspaceDir := filepath.Join(artifactDir, "workspace")

	_ = os.MkdirAll(artifactDir, 0o755)
	writeJSON(filepath.Join(artifactDir, "meta.json"), runMeta{Config: j.config.Name, Run: j.run, Model: j.config.Model})
	opts.log("▶ %-18s démarrage (%s)", rel, j.config.Model)
	opts.agent(AgentEvent{Agent: rel, Status: "running"})

	// A degenerate attempt — one that made no tool call or produced no diff,
	// e.g. the model emitting a subagent call as plain text and ending — is a
	// false success that would pollute the config's aggregates. Retry it a
	// bounded number of times to absorb such transient flubs before recording
	// it as a no-op.
	var res report.RunReport
	attempts := 1 + s.RetryCount()
	for attempt := 1; attempt <= attempts; attempt++ {
		res = attemptRun(ctx, s, j, rel, artifactDir, workspaceDir, opts)
		if res.Err != "" || !res.Degenerate() {
			break
		}
		if attempt < attempts {
			opts.log("↻ %-18s run sans effet (0 outil / diff vide), relance %d/%d", rel, attempt, attempts-1)
		}
	}

	if s.Evaluate.Enabled() && res.OK() {
		res.Checks = runChecks(ctx, s, workspaceDir, opts)
		writeJSON(filepath.Join(artifactDir, "checks.json"), res.Checks)
	}

	switch {
	case res.Err != "":
		opts.log("✗ %-18s erreur: %s", rel, firstLine(res.Err))
		opts.agent(AgentEvent{Agent: rel, Status: "failed"})
	case res.Degenerate():
		opts.log("⚠ %-18s sans effet après %d tentative(s) (écarté des classements)", rel, attempts)
		opts.agent(AgentEvent{Agent: rel, Status: "degenerate"})
	default:
		opts.log("✓ %-18s %s · $%.4f · %d fichier(s) (+%d/-%d)", rel,
			seconds(res.Metrics.DurationMS), res.Metrics.TotalCostUSD,
			res.Diff.FilesChanged, res.Diff.Insertions, res.Diff.Deletions)
		opts.agent(AgentEvent{Agent: rel, Status: "done"})
	}
	return res
}

// attemptRun prepares a fresh workspace and runs Claude once, capturing the
// metrics and diff. The workspace is wiped first so a retry starts from a clean
// copy of the app rather than the previous attempt's tree.
func attemptRun(ctx context.Context, s *spec.Spec, j job, rel, artifactDir, workspaceDir string, opts Options) report.RunReport {
	res := report.RunReport{
		Config:      j.config.Name,
		Run:         j.run,
		Model:       j.config.Model,
		ArtifactDir: rel,
	}

	if err := os.RemoveAll(workspaceDir); err != nil {
		res.Err = err.Error()
		return res
	}
	if err := workspace.Prepare(s.App, j.config.Bundle, workspaceDir, s.KeepBaseConfig); err != nil {
		res.Err = err.Error()
		return res
	}

	metrics, err := runClaude(ctx, s, j, rel, artifactDir, workspaceDir, opts)
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
		res.Files = collectFiles(workspaceDir, diff.Patch)
		_ = os.WriteFile(filepath.Join(artifactDir, "diff.patch"), []byte(diff.Patch), 0o644)
	}
	return res
}

// runChecks runs every declared acceptance check against a config's produced
// workspace and returns their outcomes.
func runChecks(ctx context.Context, s *spec.Spec, workspaceDir string, opts Options) []report.Check {
	out := make([]report.Check, 0, len(s.Evaluate.Checks))
	for _, c := range s.Evaluate.Checks {
		out = append(out, runCheck(ctx, c, workspaceDir, opts))
	}
	return out
}

// runCheck evaluates a single check: a file check stats a path on the host, a
// run check executes the command in the sandbox and passes when it exits 0.
func runCheck(ctx context.Context, c spec.Check, workspaceDir string, opts Options) report.Check {
	res := report.Check{Name: c.Name}
	if c.File != "" {
		if _, err := os.Stat(filepath.Join(workspaceDir, filepath.FromSlash(c.File))); err == nil {
			res.Passed = true
			res.Detail = "fichier présent"
		} else {
			res.Detail = "fichier absent"
		}
		return res
	}

	var buf bytes.Buffer
	err := opts.Docker.Run(ctx, docker.RunSpec{
		Image:      opts.Image,
		WorkDir:    workspaceDir,
		Entrypoint: "sh",
		// `sh -c`, not `-lc`: a login shell sources /etc/profile which resets
		// PATH and drops the image's toolchain (e.g. /usr/local/go/bin).
		Args: []string{"-c", c.Run},
	}, &buf, &buf)
	if err == nil {
		res.Passed = true
		return res
	}
	res.Detail = oneLineTail(buf.String(), 160)
	return res
}

// oneLineTail returns the last max characters of s, flattened to a single line,
// so a failing command's output stays legible in the report.
func oneLineTail(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " / ")
	if len(s) > max {
		s = "…" + s[len(s)-max:]
	}
	return s
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

func runClaude(ctx context.Context, s *spec.Spec, j job, rel, artifactDir, workspaceDir string, opts Options) (claude.Metrics, error) {
	transcript, err := os.Create(filepath.Join(artifactDir, "transcript.jsonl"))
	if err != nil {
		return claude.Metrics{}, err
	}
	defer func() { _ = transcript.Close() }()

	logFile, err := os.Create(filepath.Join(artifactDir, "stdout.log"))
	if err != nil {
		return claude.Metrics{}, err
	}
	defer func() { _ = logFile.Close() }()

	runSpec := docker.RunSpec{
		Image:     opts.Image,
		WorkDir:   workspaceDir,
		CredsFile: s.CredsFile,
		Env:       map[string]string{"DISABLE_AUTOUPDATER": "1"},
		Args:      claudeArgs(j.config),
	}
	// Tee the transcript through a live renderer so the dashboard can show each
	// agent's output as it streams, while the raw stream-json is still persisted.
	live := claude.NewLiveWriter(func(line string) {
		opts.agent(AgentEvent{Agent: rel, Line: line})
	})
	runErr := opts.Docker.Run(ctx, runSpec, io.MultiWriter(transcript, live), logFile)
	live.Flush()

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

// headlessSystemPrompt forces autonomous behaviour: a benchmark run is
// non-interactive, so an agent that stops to ask a clarifying question makes no
// tool call and produces an empty diff — a degenerate run. Telling it up front
// that no human can answer keeps a weakly-guided config (e.g. a neutral prompt
// with no skill) acting instead of asking. It is applied to every config, so it
// does not bias the comparison.
const headlessSystemPrompt = "Tu t'exécutes en mode non interactif : aucune réponse humaine n'est possible. Ne pose jamais de question de clarification et ne t'interromps pas pour demander une validation ; prends les hypothèses raisonnables nécessaires et mène la tâche à son terme en modifiant les fichiers."

func claudeArgs(c spec.Config) []string {
	return []string{
		"-p", c.Prompt,
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"--setting-sources", "project,local",
		"--append-system-prompt", headlessSystemPrompt,
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

func anyOK(runs []report.RunReport) bool {
	for _, r := range runs {
		if r.OK() {
			return true
		}
	}
	return false
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
