// Package report aggregates per-run results into Markdown and HTML documents
// comparing the benchmarked configurations side by side.
package report

import (
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
	ArtifactDir string
	Err         string
}

// Report is the aggregate of every run in a benchmark.
type Report struct {
	Prompt      string
	App         string
	GeneratedAt string
	Runs        []RunReport
}

// WriteMarkdown renders the report as Markdown.
func WriteMarkdown(w io.Writer, r Report) error {
	return markdownTemplate.Execute(w, r)
}

// WriteHTML renders the report as a self-contained HTML page.
func WriteHTML(w io.Writer, r Report) error {
	return htmlTemplate.Execute(w, r)
}
