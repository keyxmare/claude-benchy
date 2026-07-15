// Package report aggregates per-run results into Markdown and HTML documents
// comparing the benchmarked configurations side by side.
package report

import (
	"fmt"
	"io"

	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/diffcap"
)

// RunReport is the outcome of a single config run.
type RunReport struct {
	Config      string
	Run         int
	Model       string
	Metrics     claude.Metrics
	Diff        diffcap.Stats
	Patch       string
	ArtifactDir string
	Err         string
}

// OK reports whether the run succeeded end to end.
func (r RunReport) OK() bool {
	return r.Err == "" && !r.Metrics.IsError
}

// Report is the aggregate of every run in a benchmark.
type Report struct {
	Prompt      string
	App         string
	GeneratedAt string
	Runs        []RunReport
}

// Synthesis returns cross-cutting highlights computed from the run metrics.
// It is deterministic and derives only from numbers already collected.
func (r Report) Synthesis() []string {
	var ok []RunReport
	for _, run := range r.Runs {
		if run.OK() {
			ok = append(ok, run)
		}
	}

	var out []string
	if len(ok) == 0 {
		return []string{"Aucun run réussi : voir les erreurs par config."}
	}

	cheapest := pick(ok, func(a, b RunReport) bool { return a.Metrics.TotalCostUSD < b.Metrics.TotalCostUSD })
	out = append(out, fmt.Sprintf("Plus économique : **%s** ($%.4f).", cheapest.Config, cheapest.Metrics.TotalCostUSD))

	fastest := pick(ok, func(a, b RunReport) bool { return a.Metrics.DurationMS < b.Metrics.DurationMS })
	out = append(out, fmt.Sprintf("Plus rapide : **%s** (%s).", fastest.Config, seconds(fastest.Metrics.DurationMS)))

	biggest := pick(ok, func(a, b RunReport) bool { return changeSize(a) > changeSize(b) })
	out = append(out, fmt.Sprintf("Changements les plus étendus : **%s** (%d fichier(s), +%d/-%d).",
		biggest.Config, biggest.Diff.FilesChanged, biggest.Diff.Insertions, biggest.Diff.Deletions))

	if failed := failedConfigs(r.Runs); len(failed) > 0 {
		out = append(out, fmt.Sprintf("Runs en échec : %s.", join(failed)))
	}
	return out
}

func changeSize(r RunReport) int { return r.Diff.Insertions + r.Diff.Deletions }

func pick(runs []RunReport, less func(a, b RunReport) bool) RunReport {
	best := runs[0]
	for _, r := range runs[1:] {
		if less(r, best) {
			best = r
		}
	}
	return best
}

func failedConfigs(runs []RunReport) []string {
	var out []string
	for _, r := range runs {
		if !r.OK() {
			out = append(out, r.Config)
		}
	}
	return out
}

func join(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

// WriteMarkdown renders the report as Markdown.
func WriteMarkdown(w io.Writer, r Report) error {
	return markdownTemplate.Execute(w, r)
}

// WriteHTML renders the report as a self-contained HTML page.
func WriteHTML(w io.Writer, r Report) error {
	return htmlTemplate.Execute(w, r)
}
