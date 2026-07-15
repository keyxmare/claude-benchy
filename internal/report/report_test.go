package report

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/diffcap"
)

func sample() Report {
	return Report{
		Prompt:      "Add a /health endpoint.",
		App:         "/tmp/app",
		GeneratedAt: "2026-07-15 10:00:00 UTC",
		Runs: []RunReport{
			{
				Config: "baseline", Run: 1, Model: "sonnet",
				ArtifactDir: "baseline",
				Metrics: claude.Metrics{
					NumTurns: 3, ToolUses: 4, TotalCostUSD: 0.0123,
					DurationMS: 8200, Result: "Ajout de `GET /health`.",
				},
				Diff:  diffcap.Stats{FilesChanged: 1, Insertions: 3, Deletions: 0},
				Patch: "diff --git a/server.js b/server.js\n@@ -1,2 +1,5 @@\n context\n+added line\n-removed line",
			},
			{
				Config: "strict", Run: 1, Model: "opus",
				ArtifactDir: "strict",
				Err:         "docker run: exit status 1",
			},
		},
	}
}

func TestGolden(t *testing.T) {
	cases := []struct {
		name  string
		write func(w *bytes.Buffer) error
	}{
		{"report.md", func(w *bytes.Buffer) error { return WriteMarkdown(w, sample()) }},
		{"report.html", func(w *bytes.Buffer) error { return WriteHTML(w, sample()) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tc.write(&buf); err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", tc.name+".golden")
			if os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.WriteFile(golden, buf.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(buf.Bytes(), want) {
				t.Errorf("%s mismatch (run with UPDATE_GOLDEN=1 to refresh)\n--- got ---\n%s", tc.name, buf.String())
			}
		})
	}
}
