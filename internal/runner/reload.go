package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/diffcap"
	"github.com/keyxmare/claude-benchy/internal/report"
)

// maxEmbedBytes caps the size of a file embedded in the report so a stray large
// artifact cannot bloat the HTML.
const maxEmbedBytes = 256 * 1024

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

	rep := report.Report{
		Prompt:      prompt,
		App:         app,
		GeneratedAt: generatedAt,
		Runs:        runs,
	}
	var eval report.Evaluation
	if readJSON(filepath.Join(outputRoot, "evaluation.json"), &eval) == nil {
		rep.Evaluation = &eval
	}
	return rep, nil
}

func reloadRun(dir, rel string) report.RunReport {
	res := report.RunReport{Config: rel, ArtifactDir: rel}

	var meta runMeta
	if readJSON(filepath.Join(dir, "meta.json"), &meta) == nil && meta.Config != "" {
		res.Config = meta.Config
		res.Run = meta.Run
		res.Model = meta.Model
		res.Bundle = meta.Bundle
	}

	var metrics claude.Metrics
	if readJSON(filepath.Join(dir, "result.json"), &metrics) == nil {
		res.Metrics = metrics
	}

	if patch, err := os.ReadFile(filepath.Join(dir, "diff.patch")); err == nil {
		res.Patch = string(patch)
		res.Diff = diffcap.StatsFromPatch(res.Patch)
		res.Files = collectFiles(filepath.Join(dir, "workspace"), res.Patch)
	}

	var checks []report.Check
	if readJSON(filepath.Join(dir, "checks.json"), &checks) == nil {
		res.Checks = checks
	}
	return res
}

// collectFiles reads the final content of every file touched by patch from the
// (post-run) workspace, so the report can show whole files side by side. A
// deleted or missing file yields empty content on that side.
func collectFiles(workspaceDir, patch string) []report.FileVersion {
	var files []report.FileVersion
	seen := map[string]bool{}
	for _, line := range strings.Split(patch, "\n") {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		path, ok := postImagePath(line)
		if !ok || seen[path] {
			continue
		}
		seen[path] = true
		files = append(files, report.FileVersion{
			Path:    path,
			Content: readEmbed(filepath.Join(workspaceDir, filepath.FromSlash(path))),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

// postImagePath extracts the post-image path from a `diff --git a/X b/X` line.
func postImagePath(diffLine string) (string, bool) {
	i := strings.LastIndex(diffLine, " b/")
	if i < 0 {
		return "", false
	}
	return strings.TrimSpace(diffLine[i+len(" b/"):]), true
}

func readEmbed(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if info.Size() > maxEmbedBytes {
		return fmt.Sprintf("… fichier trop volumineux pour l'aperçu (%d octets)\n", info.Size())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if !utf8.Valid(b) {
		return "… contenu binaire non affiché\n"
	}
	return string(b)
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
