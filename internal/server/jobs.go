package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/keyxmare/claude-benchy/internal/docker"
	"github.com/keyxmare/claude-benchy/internal/report"
	"github.com/keyxmare/claude-benchy/internal/runner"
	"github.com/keyxmare/claude-benchy/internal/spec"
)

const (
	timestampLayout   = "20060102-150405"
	generatedAtLayout = "2006-01-02 15:04:05 MST"
)

type jobStatus string

const (
	statusRunning jobStatus = "running"
	statusDone    jobStatus = "done"
	statusFailed  jobStatus = "failed"
	statusStopped jobStatus = "stopped"
)

// job is a single benchmark execution tracked in memory so the browser can
// follow its progress live. Its artifacts are written to disk under
// outputRoot; the in-memory state (logs, status) is lost on server restart but
// the results remain listable through the history.
// streamEvent is one entry of a job's ordered progress stream: an orchestration
// log line (Kind "log") or a per-agent update (Kind "agent").
type streamEvent struct {
	Kind   string `json:"-"`
	Agent  string `json:"agent,omitempty"`
	Line   string `json:"line,omitempty"`
	Status string `json:"status,omitempty"`
}

type job struct {
	id         string
	outputRoot string
	cancel     context.CancelFunc

	mu      sync.Mutex
	status  jobStatus
	events  []streamEvent
	errMsg  string
	changed chan struct{} // closed (and replaced) on every log/status change
}

func newJob(id, outputRoot string, cancel context.CancelFunc) *job {
	return &job{
		id:         id,
		outputRoot: outputRoot,
		cancel:     cancel,
		status:     statusRunning,
		changed:    make(chan struct{}),
	}
}

// append records an orchestration progress line. It matches runner.Options.Log
// and may be called concurrently from several goroutines.
func (j *job) append(line string) {
	j.record(streamEvent{Kind: "log", Line: line})
}

// agent records a per-agent update. It matches runner.Options.Agent.
func (j *job) agent(ev runner.AgentEvent) {
	j.record(streamEvent{Kind: "agent", Agent: ev.Agent, Line: ev.Line, Status: string(ev.Status)})
}

func (j *job) record(ev streamEvent) {
	j.mu.Lock()
	j.events = append(j.events, ev)
	j.signalLocked()
	j.mu.Unlock()
}

func (j *job) finish(status jobStatus, errMsg string) {
	j.mu.Lock()
	j.status = status
	j.errMsg = errMsg
	j.signalLocked()
	j.mu.Unlock()
}

// signalLocked wakes every waiter by closing the current change channel and
// installing a fresh one. Caller must hold j.mu.
func (j *job) signalLocked() {
	close(j.changed)
	j.changed = make(chan struct{})
}

// snapshot returns the events from index i onward, the current status and
// error, and a channel that closes when the job next changes — the primitive
// the SSE handler waits on.
func (j *job) snapshot(i int) (events []streamEvent, status jobStatus, errMsg string, changed chan struct{}) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if i < len(j.events) {
		events = append(events, j.events[i:]...)
	}
	return events, j.status, j.errMsg, j.changed
}

// jobManager owns the live jobs and hands out unique ids and output directories.
type jobManager struct {
	mu   sync.Mutex
	jobs map[string]*job
	seq  int
}

func newJobManager() *jobManager {
	return &jobManager{jobs: map[string]*job{}}
}

func (m *jobManager) get(id string) (*job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	return j, ok
}

// start reserves an output directory, registers a job and launches the run in
// the background. It returns the job so the caller can redirect to its page.
func (m *jobManager) start(s *spec.Spec, rawYAML, image string, runnerDocker docker.Runner) (*job, error) {
	m.mu.Lock()
	root, err := reserveDir(s.Output)
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}
	m.seq++
	id := strconv.Itoa(m.seq)
	ctx, cancel := context.WithCancel(context.Background())
	j := newJob(id, root, cancel)
	m.jobs[id] = j
	m.mu.Unlock()

	go m.exec(ctx, j, s, rawYAML, image, runnerDocker)
	return j, nil
}

func (m *jobManager) exec(ctx context.Context, j *job, s *spec.Spec, rawYAML, image string, runnerDocker docker.Runner) {
	if rawYAML != "" {
		_ = os.WriteFile(filepath.Join(j.outputRoot, "bench.yaml"), []byte(rawYAML), 0o644)
	}

	rep, err := runner.Run(ctx, s, j.outputRoot, time.Now().Format(generatedAtLayout), runner.Options{
		Image:  image,
		Docker: runnerDocker,
		Log:    j.append,
		Agent:  j.agent,
	})
	if err != nil {
		j.finish(statusFailed, err.Error())
		return
	}
	if err := report.Write(j.outputRoot, rep); err != nil {
		j.finish(statusFailed, err.Error())
		return
	}
	if ctx.Err() != nil {
		j.finish(statusStopped, "run interrompu")
		return
	}
	j.finish(statusDone, "")
}

// reserveDir atomically claims a fresh timestamped directory under output,
// suffixing a counter should two runs land in the same second.
func reserveDir(output string) (string, error) {
	if err := os.MkdirAll(output, 0o755); err != nil {
		return "", err
	}
	base := filepath.Join(output, time.Now().Format(timestampLayout))
	root := base
	for i := 2; ; i++ {
		err := os.Mkdir(root, 0o755)
		if err == nil {
			return root, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
		root = fmt.Sprintf("%s-%d", base, i)
	}
}
