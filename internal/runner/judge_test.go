package runner

import (
	"strings"
	"testing"

	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/diffcap"
	"github.com/keyxmare/claude-benchy/internal/report"
)

func TestBuildJudgePromptEmbedsOKRunsAndTruncates(t *testing.T) {
	big := strings.Repeat("x", maxDiffBytes+500)
	rep := report.Report{
		Prompt: "Ajoute un endpoint /health.",
		Runs: []report.RunReport{
			{Config: "a", Metrics: claude.Metrics{Result: "fait", ToolUses: 3}, Diff: diffcap.Stats{FilesChanged: 1}, Patch: big},
			// A failed run: excluded, there is nothing to judge.
			{Config: "b", Metrics: claude.Metrics{ToolUses: 2}, Diff: diffcap.Stats{FilesChanged: 1}, Patch: "diff b", Err: "boom"},
			// A degenerate run (no tool call, no diff): excluded too.
			{Config: "c", Metrics: claude.Metrics{ToolUses: 0}, Patch: "diff c"},
		},
	}

	prompt, err := buildJudgePrompt(rep, []string{"couvre le cas nominal"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Ajoute un endpoint /health.", "couvre le cas nominal", "### a (statut : réussi", "fait"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q\n%s", want, prompt)
		}
	}
	for _, unwanted := range []string{"### b ", "### c ", "diff c"} {
		if strings.Contains(prompt, unwanted) {
			t.Errorf("non-OK run should be excluded, but prompt contains %q\n%s", unwanted, prompt)
		}
	}
	if !strings.Contains(prompt, "diff tronqué") {
		t.Error("oversized diff should be truncated")
	}
	if strings.Count(prompt, "x") >= len(big) {
		t.Error("truncation did not shrink the diff")
	}
}

func TestParseJudgeToleratesFencesAndComputesScoreRanking(t *testing.T) {
	answer := "Voici mon évaluation :\n```json\n" +
		`{"configs":[` +
		`{"label":"a","verdict":"complet","criteria":[{"criterion":"c1","level":"respecté"},{"criterion":"c2","level":"respecté"}]},` +
		`{"label":"b","verdict":"partiel","criteria":[{"criterion":"c1","level":"respecté"},{"criterion":"c2","level":"non"}]}` +
		`],"recommendation":"garder a"}` +
		"\n```\nFin."

	eval, err := parseJudge(answer)
	if err != nil {
		t.Fatal(err)
	}
	// Score is derived from criteria levels (equal weight), not from the model.
	if eval.For("a").Score != 100 || eval.For("b").Score != 50 {
		t.Errorf("scores not derived: a=%d b=%d", eval.For("a").Score, eval.For("b").Score)
	}
	// Ranking is derived from score, best first.
	if len(eval.Ranking) != 2 || eval.Ranking[0] != "a" || eval.Ranking[1] != "b" {
		t.Errorf("ranking not derived from score: %v", eval.Ranking)
	}
	if eval.Recommendation != "garder a" {
		t.Errorf("recommendation not parsed: %q", eval.Recommendation)
	}
}

func TestParseJudgeRejectsNonJSON(t *testing.T) {
	if _, err := parseJudge("je ne sais pas"); err == nil {
		t.Error("expected an error when no JSON object is present")
	}
}
