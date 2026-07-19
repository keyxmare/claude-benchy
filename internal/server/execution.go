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

type execStatus string

const (
	statusRunning execStatus = "running"
	statusDone    execStatus = "done"
	statusFailed  execStatus = "failed"
	statusStopped execStatus = "stopped"
)

// streamEvent is one entry of an execution's ordered progress stream: an
// orchestration log line (Kind "log") or a per-agent update (Kind "agent").
type streamEvent struct {
	Kind   string `json:"-"`
	Agent  string `json:"agent,omitempty"`
	Line   string `json:"line,omitempty"`
	Status string `json:"status,omitempty"`
}

// execution is a single whole-bench launch tracked in memory so the browser can
// follow its progress live. It runs every config × run of one bench; the domain
// Job (a single config × run) lives in runner. Its artifacts are written to disk
// under outputRoot; the in-memory state (logs, status) is lost on server restart
// but the results remain listable through the history.
type execution struct {
	id         string
	outputRoot string
	cancel     context.CancelFunc

	mu      sync.Mutex
	status  execStatus
	events  []streamEvent
	errMsg  string
	changed chan struct{} // closed (and replaced) on every log/status change
}

func newExecution(id, outputRoot string, cancel context.CancelFunc) *execution {
	return &execution{
		id:         id,
		outputRoot: outputRoot,
		cancel:     cancel,
		status:     statusRunning,
		changed:    make(chan struct{}),
	}
}

// append records an orchestration progress line. It matches runner.Options.Log
// and may be called concurrently from several goroutines.
func (e *execution) append(line string) {
	e.record(streamEvent{Kind: "log", Line: line})
}

// agent records a per-agent update. It matches runner.Options.Agent.
func (e *execution) agent(ev runner.AgentEvent) {
	e.record(streamEvent{Kind: "agent", Agent: ev.Agent, Line: ev.Line, Status: string(ev.Status)})
}

func (e *execution) record(ev streamEvent) {
	e.mu.Lock()
	e.events = append(e.events, ev)
	e.signalLocked()
	e.mu.Unlock()
}

func (e *execution) finish(status execStatus, errMsg string) {
	e.mu.Lock()
	e.status = status
	e.errMsg = errMsg
	e.signalLocked()
	e.mu.Unlock()
}

// signalLocked wakes every waiter by closing the current change channel and
// installing a fresh one. Caller must hold e.mu.
func (e *execution) signalLocked() {
	close(e.changed)
	e.changed = make(chan struct{})
}

// snapshot returns the events from index i onward, the current status and
// error, and a channel that closes when the execution next changes — the
// primitive the SSE handler waits on.
func (e *execution) snapshot(i int) (events []streamEvent, status execStatus, errMsg string, changed chan struct{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if i < len(e.events) {
		events = append(events, e.events[i:]...)
	}
	return events, e.status, e.errMsg, e.changed
}

// execManager owns the live executions and hands out unique ids and output
// directories.
type execManager struct {
	mu    sync.Mutex
	execs map[string]*execution
	seq   int
}

func newExecManager() *execManager {
	return &execManager{execs: map[string]*execution{}}
}

func (m *execManager) get(id string) (*execution, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.execs[id]
	return e, ok
}

// start reserves an output directory, registers an execution and launches the
// run in the background. It returns the execution so the caller can redirect to
// its page.
func (m *execManager) start(s *spec.Spec, rawYAML, image string, runnerDocker docker.Runner) (*execution, error) {
	m.mu.Lock()
	root, err := reserveDir(s.Output)
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}
	m.seq++
	id := strconv.Itoa(m.seq)
	ctx, cancel := context.WithCancel(context.Background())
	e := newExecution(id, root, cancel)
	m.execs[id] = e
	m.mu.Unlock()

	go m.exec(ctx, e, s, rawYAML, image, runnerDocker)
	return e, nil
}

func (m *execManager) exec(ctx context.Context, e *execution, s *spec.Spec, rawYAML, image string, runnerDocker docker.Runner) {
	if rawYAML != "" {
		_ = os.WriteFile(filepath.Join(e.outputRoot, "bench.yaml"), []byte(rawYAML), 0o644)
	}

	rep, err := runner.Run(ctx, s, e.outputRoot, time.Now().Format(generatedAtLayout), runner.Options{
		Image:  image,
		Docker: runnerDocker,
		Log:    e.append,
		Agent:  e.agent,
	})
	if err != nil {
		e.finish(statusFailed, err.Error())
		return
	}
	if err := report.Write(e.outputRoot, rep); err != nil {
		e.finish(statusFailed, err.Error())
		return
	}
	if ctx.Err() != nil {
		e.finish(statusStopped, "run interrompu")
		return
	}
	e.finish(statusDone, "")
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
