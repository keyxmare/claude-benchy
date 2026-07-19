package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// formFromDir rebuilds the dashboard form from a benchmark output directory so
// its configuration can be reviewed, adapted and re-launched, in decreasing
// order of fidelity:
//  1. the run's own persisted bench.yaml (dashboard runs, and CLI runs going
//     forward) — exact;
//  2. the nearest source bench.yaml above the run directory (legacy CLI runs)
//     — the config that most likely produced it;
//  3. reconstruction from the surviving artifacts, guessing config bundles from
//     the conventional layout under root.
func formFromDir(root, dir string) formValues {
	if d, ok := readBenchDoc(dir); ok {
		return formFromDoc(d)
	}
	if d, ok := nearestBench(filepath.Dir(dir)); ok {
		return formFromDoc(d)
	}
	return formFromArtifacts(root, dir)
}

func readBenchDoc(dir string) (benchDoc, bool) {
	b, err := os.ReadFile(filepath.Join(dir, "bench.yaml"))
	if err != nil {
		return benchDoc{}, false
	}
	var d benchDoc
	if yaml.Unmarshal(b, &d) != nil || len(d.Configs) == 0 {
		return benchDoc{}, false
	}
	return d, true
}

// nearestBench walks up from dir looking for a source bench.yaml, so a run
// produced by the CLI (which does not persist its config inside the run dir for
// older results) can still be reused from the bench that lives above its
// results directory.
func nearestBench(dir string) (benchDoc, bool) {
	cur := dir
	for i := 0; i < 6; i++ {
		if d, ok := readBenchDoc(cur); ok {
			return d, true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return benchDoc{}, false
}

func formFromDoc(d benchDoc) formValues {
	fv := formValues{
		Prompt:         d.Prompt,
		PromptFile:     d.PromptFile,
		App:            d.App,
		Model:          d.Model,
		Output:         d.Output,
		KeepBaseConfig: d.KeepBaseConfig,
	}
	if d.Runs > 0 {
		fv.Runs = strconv.Itoa(d.Runs)
	}
	if d.Concurrency > 0 {
		fv.Concurrency = strconv.Itoa(d.Concurrency)
	}
	if d.Retries != nil {
		fv.Retries = strconv.Itoa(*d.Retries)
	}
	if d.Auth != nil {
		fv.AuthConfigDir = d.Auth.ConfigDir
	}
	if d.Sandbox != nil {
		fv.SandboxImage = d.Sandbox.Image
	}
	if d.Evaluate != nil {
		fv.EvalModel = d.Evaluate.Model
		fv.Rubric = strings.Join(d.Evaluate.Rubric, "\n")
		for _, c := range d.Evaluate.Checks {
			fv.Checks = append(fv.Checks, checkRow(c))
		}
	}
	for _, c := range d.Configs {
		fv.Configs = append(fv.Configs, configRow(c))
	}
	return withDefaults(fv)
}

func formFromArtifacts(root, dir string) formValues {
	fv := formValues{}

	var meta struct {
		Prompt string `json:"prompt"`
		App    string `json:"app"`
	}
	if b, err := os.ReadFile(filepath.Join(dir, "bench.json")); err == nil {
		_ = json.Unmarshal(b, &meta)
	}
	if meta.Prompt != "(varies per config)" {
		fv.Prompt = meta.Prompt
	}
	fv.App = meta.App

	// Config names and models survive in each config's meta.json; the bundle
	// path does not. As a last resort, guess it from the conventional
	// ./configs/<name> layout under root, but only when that directory exists —
	// otherwise the field stays empty for the user to fill in.
	if ents, err := os.ReadDir(dir); err == nil {
		for _, e := range ents {
			if !e.IsDir() || e.Name() == ".judge" {
				continue
			}
			name, model := configMeta(filepath.Join(dir, e.Name()))
			if name == "" {
				name = e.Name()
			}
			fv.Configs = append(fv.Configs, configRow{Name: name, Bundle: guessBundle(root, name), Model: model})
		}
	}

	// The rubric survives in evaluation.json; the checks' commands do not.
	var eval struct {
		Rubric []string `json:"rubric"`
	}
	if b, err := os.ReadFile(filepath.Join(dir, "evaluation.json")); err == nil {
		if json.Unmarshal(b, &eval) == nil {
			fv.Rubric = strings.Join(eval.Rubric, "\n")
		}
	}
	return withDefaults(fv)
}

// guessBundle returns the conventional ./configs/<name> bundle path when that
// directory exists under root, or an empty string otherwise.
func guessBundle(root, name string) string {
	rel := filepath.Join("configs", name)
	if info, err := os.Stat(filepath.Join(root, rel)); err == nil && info.IsDir() {
		return "./" + filepath.ToSlash(rel)
	}
	return ""
}

// configMeta reads a config's name and model from its meta.json, looking one
// level deeper when the config was repeated (runs > 1 writes run-N/meta.json).
func configMeta(configDir string) (name, model string) {
	var m struct {
		Config string `json:"config"`
		Model  string `json:"model"`
	}
	if b, err := os.ReadFile(filepath.Join(configDir, "meta.json")); err == nil {
		_ = json.Unmarshal(b, &m)
		return m.Config, m.Model
	}
	if ents, err := os.ReadDir(configDir); err == nil {
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			if b, err := os.ReadFile(filepath.Join(configDir, e.Name(), "meta.json")); err == nil {
				_ = json.Unmarshal(b, &m)
				return m.Config, m.Model
			}
		}
	}
	return "", ""
}

// withDefaults fills the fields the dashboard always expects to be present.
func withDefaults(fv formValues) formValues {
	if fv.Model == "" {
		fv.Model = "sonnet"
	}
	if fv.Runs == "" {
		fv.Runs = "1"
	}
	if fv.Concurrency == "" {
		fv.Concurrency = "1"
	}
	if fv.Output == "" {
		fv.Output = "results"
	}
	if len(fv.Configs) == 0 {
		fv.Configs = []configRow{{}}
	}
	return fv
}
