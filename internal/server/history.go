package server

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// historyEntry summarises one benchmark output directory for the history list.
type historyEntry struct {
	Dir     string // path relative to root, used as the ?dir= parameter
	Name    string // the output directory's base name (the timestamp)
	Prompt  string
	App     string
	Model   string
	Runs    int
	Configs []string // config names, when a bench.yaml was persisted
	Count   int      // number of config directories on disk
	HasEval bool
}

// scanHistory walks root and returns every benchmark output directory (marked
// by a bench.json), newest first. It prunes version-control and workspace
// subtrees and never descends into a run directory it has already recognised.
func scanHistory(root string) []historyEntry {
	var entries []historyEntry
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		switch d.Name() {
		case ".git", "node_modules", "workspace":
			return filepath.SkipDir
		}
		if _, statErr := os.Stat(filepath.Join(path, "bench.json")); statErr != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return filepath.SkipDir
		}
		entries = append(entries, loadEntry(path, rel))
		return filepath.SkipDir // a bench output dir is a leaf for our purposes
	})
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name != entries[j].Name {
			return entries[i].Name > entries[j].Name
		}
		return entries[i].Dir < entries[j].Dir
	})
	return entries
}

func loadEntry(path, rel string) historyEntry {
	e := historyEntry{Dir: rel, Name: filepath.Base(path)}

	var meta struct {
		Prompt string `json:"prompt"`
		App    string `json:"app"`
	}
	if b, err := os.ReadFile(filepath.Join(path, "bench.json")); err == nil {
		_ = json.Unmarshal(b, &meta)
	}
	e.Prompt, e.App = meta.Prompt, meta.App

	if b, err := os.ReadFile(filepath.Join(path, "bench.yaml")); err == nil {
		var y struct {
			Model   string `yaml:"model"`
			Runs    int    `yaml:"runs"`
			Configs []struct {
				Name string `yaml:"name"`
			} `yaml:"configs"`
		}
		if yaml.Unmarshal(b, &y) == nil {
			e.Model, e.Runs = y.Model, y.Runs
			for _, c := range y.Configs {
				e.Configs = append(e.Configs, c.Name)
			}
		}
	}

	if _, err := os.Stat(filepath.Join(path, "evaluation.json")); err == nil {
		e.HasEval = true
	}
	e.Count = countConfigs(path)
	return e
}

// countConfigs counts the config directories written under a benchmark output
// directory (every immediate subdirectory but the judge scratch dir).
func countConfigs(path string) int {
	ents, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, ent := range ents {
		if ent.IsDir() && ent.Name() != ".judge" {
			n++
		}
	}
	return n
}
