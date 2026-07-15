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
	defaultOutput      = "results"
	credentialsFile    = ".credentials.json"
)

// Auth describes how Claude authenticates inside the sandbox container.
type Auth struct {
	// ConfigDir is the host directory holding the OAuth credentials to mount.
	ConfigDir string `yaml:"configDir"`
}

// Config is a single Claude configuration to benchmark.
type Config struct {
	Name       string `yaml:"name"`
	Bundle     string `yaml:"bundle"`
	Model      string `yaml:"model"`
	Prompt     string `yaml:"prompt"`
	PromptFile string `yaml:"promptFile"`
}

// Spec is a full benchmark definition.
type Spec struct {
	Prompt      string   `yaml:"prompt"`
	PromptFile  string   `yaml:"promptFile"`
	App         string   `yaml:"app"`
	Model       string   `yaml:"model"`
	Runs        int      `yaml:"runs"`
	Concurrency int      `yaml:"concurrency"`
	Auth        Auth     `yaml:"auth"`
	Output      string   `yaml:"output"`
	Configs     []Config `yaml:"configs"`

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
	if err := s.resolve(baseDir); err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	return &s, nil
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
	if s.Output == "" {
		s.Output = defaultOutput
	}
	if s.Auth.ConfigDir == "" {
		s.Auth.ConfigDir = defaultConfigDir()
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
	s.CredsFile = filepath.Join(configDir, credentialsFile)

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
