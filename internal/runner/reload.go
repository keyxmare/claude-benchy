package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/diffcap"
	"github.com/keyxmare/claude-benchy/internal/report"
)

type benchMeta struct {
	Prompt string `json:"prompt"`
	App    string `json:"app"`
}

// BenchInfo returns the prompt and app recorded for a run directory, if any.
func BenchInfo(outputRoot string) (prompt, app string) {
	var m benchMeta
	_ = readJSON(filepath.Join(outputRoot, "bench.json"), &m)
	return m.Prompt, m.App
}

// Reload rebuilds a report from the artifacts already written under outputRoot,
// without re-running any benchmark. It lets a report be re-rendered (e.g. after
// a template change) for free.
func Reload(outputRoot, app, prompt, generatedAt string) (report.Report, error) {
	var runs []report.RunReport
	err := filepath.WalkDir(outputRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if _, statErr := os.Stat(filepath.Join(path, "diff.patch")); statErr != nil {
			return nil
		}
		rel, err := filepath.Rel(outputRoot, path)
		if err != nil {
			return err
		}
		runs = append(runs, reloadRun(path, rel))
		return filepath.SkipDir
	})
	if err != nil {
		return report.Report{}, err
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].ArtifactDir < runs[j].ArtifactDir })
	return report.Report{
		Prompt:      prompt,
		App:         app,
		GeneratedAt: generatedAt,
		Runs:        runs,
	}, nil
}

func reloadRun(dir, rel string) report.RunReport {
	res := report.RunReport{Config: rel, ArtifactDir: rel}

	var meta runMeta
	if readJSON(filepath.Join(dir, "meta.json"), &meta) == nil && meta.Config != "" {
		res.Config = meta.Config
		res.Run = meta.Run
		res.Model = meta.Model
	}

	var metrics claude.Metrics
	if readJSON(filepath.Join(dir, "result.json"), &metrics) == nil {
		res.Metrics = metrics
	}

	if patch, err := os.ReadFile(filepath.Join(dir, "diff.patch")); err == nil {
		res.Patch = string(patch)
		res.Diff = patchStats(res.Patch)
	}
	return res
}

func patchStats(patch string) diffcap.Stats {
	var s diffcap.Stats
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			s.FilesChanged++
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			// file headers, ignore
		case strings.HasPrefix(line, "+"):
			s.Insertions++
		case strings.HasPrefix(line, "-"):
			s.Deletions++
		}
	}
	return s
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
