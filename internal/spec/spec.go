// Package spec loads and validates a benchmark specification file.
package spec

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultModel       = "sonnet"
	defaultRuns        = 1
	defaultConcurrency = 1
	defaultRetries     = 2
	defaultOutput      = "results"
	credentialsFile    = ".credentials.json"
)

// Auth describes how Claude authenticates inside the sandbox container.
type Auth struct {
	// ConfigDir is the host directory holding the OAuth credentials to mount.
	ConfigDir string `yaml:"configDir"`
}

// Sandbox tunes the container in which Claude runs.
type Sandbox struct {
	// Image overrides the default sandbox image (e.g. one bundling the
	// project's own toolchain so Claude can run its quality gates in-sandbox).
	Image string `yaml:"image"`
}

// Config is a single Claude configuration to benchmark.
type Config struct {
	Name       string `yaml:"name"`
	Bundle     string `yaml:"bundle"`
	Model      string `yaml:"model"`
	Prompt     string `yaml:"prompt"`
	PromptFile string `yaml:"promptFile"`
}

// Check is a deterministic acceptance check run in the sandbox against each
// config's produced workspace. It passes when the command exits 0 (Run) or the
// path exists (File).
type Check struct {
	Name string `yaml:"name"`
	Run  string `yaml:"run"`
	File string `yaml:"file"`
}

// Evaluate configures the post-run evaluation of how well each config met the
// expectation: deterministic acceptance checks plus an LLM judge that scores
// the produced changes against a rubric. It is optional; when neither rubric
// nor checks are set, no evaluation runs.
type Evaluate struct {
	Model  string   `yaml:"model"`
	Rubric []string `yaml:"rubric"`
	Checks []Check  `yaml:"checks"`
}

// Enabled reports whether any evaluation was requested.
func (e Evaluate) Enabled() bool {
	return len(e.Rubric) > 0 || len(e.Checks) > 0
}

// RetryCount returns the number of extra attempts a degenerate run gets,
// falling back to the default when unset.
func (s *Spec) RetryCount() int {
	if s.Retries == nil {
		return defaultRetries
	}
	return *s.Retries
}

// Spec is a full benchmark definition.
type Spec struct {
	Prompt      string `yaml:"prompt"`
	PromptFile  string `yaml:"promptFile"`
	App         string `yaml:"app"`
	Model       string `yaml:"model"`
	Runs        int    `yaml:"runs"`
	Concurrency int    `yaml:"concurrency"`
	// Retries is how many extra attempts a degenerate run (one that made no tool
	// call or produced no diff) gets before being recorded as a failed run. It
	// absorbs transient flubs — e.g. the model emitting a tool call as plain
	// text and ending — that would otherwise pollute a config's results. A nil
	// value means the default; an explicit 0 disables retries.
	Retries  *int     `yaml:"retries"`
	Auth     Auth     `yaml:"auth"`
	Sandbox  Sandbox  `yaml:"sandbox"`
	Output   string   `yaml:"output"`
	Evaluate Evaluate `yaml:"evaluate"`
	Configs  []Config `yaml:"configs"`

	// KeepBaseConfig keeps the app's own Claude config as the base and layers
	// each bundle on top of it: a colliding CLAUDE.md is appended to the app's
	// rather than replacing it, so a bundle adds instructions instead of
	// discarding the project's own. When false (default) a bundle file wins.
	KeepBaseConfig bool `yaml:"keepBaseConfig"`

	// CredsFile is the resolved absolute path to the credentials file to mount.
	CredsFile string `yaml:"-"`
}

// Load reads, resolves and validates the spec at path. All relative paths in
// the spec are resolved against the directory containing the spec file.
func Load(path string) (*Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}

	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}

	baseDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	if err := s.Build(baseDir); err != nil {
		return nil, err
	}
	return &s, nil
}

// Build resolves the spec's relative paths against baseDir and validates it in
// place, so a Spec assembled in memory (e.g. from a web form) goes through the
// same resolution and checks as one loaded from a file.
func (s *Spec) Build(baseDir string) error {
	if err := s.resolve(baseDir); err != nil {
		return err
	}
	return s.validate()
}

func (s *Spec) resolve(baseDir string) error {
	if s.Model == "" {
		s.Model = defaultModel
	}
	if s.Runs == 0 {
		s.Runs = defaultRuns
	}
	if s.Concurrency == 0 {
		s.Concurrency = defaultConcurrency
	}
	if s.Retries == nil {
		n := defaultRetries
		s.Retries = &n
	}
	if s.Output == "" {
		s.Output = defaultOutput
	}
	if s.Auth.ConfigDir == "" {
		s.Auth.ConfigDir = defaultConfigDir()
	}
	if s.Evaluate.Model == "" {
		s.Evaluate.Model = s.Model
	}

	s.App = resolvePath(baseDir, s.App)
	s.Output = resolvePath(baseDir, s.Output)
	if s.PromptFile != "" {
		s.PromptFile = resolvePath(baseDir, s.PromptFile)
	}

	configDir, err := expandHome(s.Auth.ConfigDir)
	if err != nil {
		return err
	}
	s.Auth.ConfigDir = configDir
	s.CredsFile = resolveCredsFile(configDir)

	basePrompt, err := s.resolvedPrompt(s.Prompt, s.PromptFile)
	if err != nil {
		return err
	}

	for i := range s.Configs {
		c := &s.Configs[i]
		c.Bundle = resolvePath(baseDir, c.Bundle)
		if c.PromptFile != "" {
			c.PromptFile = resolvePath(baseDir, c.PromptFile)
		}
		prompt, err := s.resolvedPrompt(c.Prompt, c.PromptFile)
		if err != nil {
			return fmt.Errorf("config %q: %w", c.Name, err)
		}
		if prompt == "" {
			prompt = basePrompt
		}
		c.Prompt = prompt
		if c.Model == "" {
			c.Model = s.Model
		}
	}
	return nil
}

func (s *Spec) resolvedPrompt(inline, file string) (string, error) {
	if inline != "" && file != "" {
		return "", fmt.Errorf("prompt and promptFile are mutually exclusive")
	}
	if inline != "" {
		return inline, nil
	}
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read promptFile: %w", err)
		}
		return string(b), nil
	}
	return "", nil
}

func (s *Spec) validate() error {
	if s.App == "" {
		return fmt.Errorf("app is required")
	}
	if info, err := os.Stat(s.App); err != nil || !info.IsDir() {
		return fmt.Errorf("app %q is not a directory", s.App)
	}
	if len(s.Configs) == 0 {
		return fmt.Errorf("at least one config is required")
	}
	if s.Concurrency < 1 {
		return fmt.Errorf("concurrency must be >= 1")
	}
	if s.Runs < 1 {
		return fmt.Errorf("runs must be >= 1")
	}
	if s.Retries != nil && *s.Retries < 0 {
		return fmt.Errorf("retries must be >= 0")
	}

	for i, c := range s.Evaluate.Checks {
		if c.Name == "" {
			return fmt.Errorf("evaluate.checks[%d]: name is required", i)
		}
		if c.Run == "" && c.File == "" {
			return fmt.Errorf("evaluate check %q: set run or file", c.Name)
		}
	}

	seen := make(map[string]bool, len(s.Configs))
	for _, c := range s.Configs {
		if c.Name == "" {
			return fmt.Errorf("every config needs a name")
		}
		if seen[c.Name] {
			return fmt.Errorf("duplicate config name %q", c.Name)
		}
		seen[c.Name] = true
		if info, err := os.Stat(c.Bundle); err != nil || !info.IsDir() {
			return fmt.Errorf("config %q: bundle %q is not a directory", c.Name, c.Bundle)
		}
		if c.Prompt == "" {
			return fmt.Errorf("config %q: no prompt (set a top-level prompt/promptFile or one per config)", c.Name)
		}
	}
	return nil
}

func resolvePath(baseDir, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(baseDir, p)
}
