package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/diffcap"
	"github.com/keyxmare/claude-benchy/internal/docker"
	"github.com/keyxmare/claude-benchy/internal/report"
	"github.com/keyxmare/claude-benchy/internal/spec"
)

// validJudgeJSON is a well-formed judge answer for one config against one
// criterion.
const validJudgeJSON = `{"configs":[{"label":"a","verdict":"ok","criteria":[{"criterion":"c1","level":"respecté"}]}],"recommendation":"r"}`

// writeJudgeAnswer emits a stream-json result event whose text is answer, as the
// judge sandbox would.
func writeJudgeAnswer(stdout io.Writer, answer string) {
	_, _ = fmt.Fprintf(stdout, `{"type":"result","subtype":"success","is_error":false,"result":%q}`+"\n", answer)
}

func judgeSpec() *spec.Spec {
	return &spec.Spec{Evaluate: spec.Evaluate{Model: "sonnet", Rubric: []string{"c1"}}}
}

func okReport() report.Report {
	return report.Report{
		Prompt: "p",
		Runs: []report.RunReport{
			{Config: "a", Metrics: claude.Metrics{Result: "ok", ToolUses: 1}, Diff: diffcap.Stats{FilesChanged: 1}, Patch: "diff"},
		},
	}
}

func TestJudgeRetriesOnInvalidJSONThenSucceeds(t *testing.T) {
	calls := 0
	fake := funcDocker{run: func(_ context.Context, _ docker.RunSpec, stdout, _ io.Writer) error {
		calls++
		if calls == 1 {
			writeJudgeAnswer(stdout, "pas de json ici")
		} else {
			writeJudgeAnswer(stdout, validJudgeJSON)
		}
		return nil
	}}
	eval, err := judge(context.Background(), judgeSpec(), okReport(), t.TempDir(), Options{Image: "img", Docker: fake})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("expected one corrective retry, got %d judge call(s)", calls)
	}
	if eval == nil || len(eval.Configs) != 1 {
		t.Errorf("expected a parsed evaluation, got %+v", eval)
	}
}

func TestJudgeReturnsParseErrorAfterRetries(t *testing.T) {
	calls := 0
	fake := funcDocker{run: func(_ context.Context, _ docker.RunSpec, stdout, _ io.Writer) error {
		calls++
		writeJudgeAnswer(stdout, "jamais du json")
		return nil
	}}
	eval, err := judge(context.Background(), judgeSpec(), okReport(), t.TempDir(), Options{Image: "img", Docker: fake})
	if err == nil {
		t.Error("expected an error when the judge never returns valid JSON")
	}
	if eval != nil {
		t.Errorf("expected no evaluation on persistent parse failure, got %+v", eval)
	}
	if calls != 2 {
		t.Errorf("expected two attempts, got %d", calls)
	}
}

func TestRunJudgeReturnsMkdirError(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// outputRoot is a regular file, so creating the .judge scratch dir fails.
	_, err := runJudge(context.Background(), judgeSpec(), "prompt", blocker, Options{Image: "img"})
	if err == nil {
		t.Error("expected an error when the judge scratch dir cannot be created")
	}
}

func TestRunJudgeReportsTranscriptParseError(t *testing.T) {
	fake := funcDocker{run: func(_ context.Context, _ docker.RunSpec, stdout, _ io.Writer) error {
		_, _ = fmt.Fprintln(stdout, "garbage without a result event")
		return nil
	}}
	_, err := runJudge(context.Background(), judgeSpec(), "prompt", t.TempDir(), Options{Image: "img", Docker: fake})
	if err == nil {
		t.Error("expected an error when the judge transcript cannot be parsed")
	}
}

func TestRunJudgeReportsJudgeError(t *testing.T) {
	fake := funcDocker{run: func(_ context.Context, _ docker.RunSpec, stdout, _ io.Writer) error {
		_, _ = fmt.Fprintln(stdout, `{"type":"result","is_error":true,"subtype":"error_max_turns"}`)
		return nil
	}}
	_, err := runJudge(context.Background(), judgeSpec(), "prompt", t.TempDir(), Options{Image: "img", Docker: fake})
	if err == nil {
		t.Error("expected an error when the judge itself reports an error")
	}
}

func TestParseJudgeRejectsMalformedJSONObject(t *testing.T) {
	// Braces are present so an object is extracted, but its content is invalid.
	if _, err := parseJudge("{not really json}"); err == nil {
		t.Error("expected a decode error for a malformed JSON object")
	}
}

func TestStatusLabel(t *testing.T) {
	cases := map[string]struct {
		run  report.RunReport
		want string
	}{
		"ok":     {report.RunReport{Metrics: claude.Metrics{ToolUses: 1}, Diff: diffcap.Stats{FilesChanged: 1}}, "réussi"},
		"failed": {report.RunReport{Err: "boom"}, "échec"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := statusLabel(tc.run); got != tc.want {
				t.Errorf("statusLabel(%+v) = %q, want %q", tc.run, got, tc.want)
			}
		})
	}
}

func TestFallback(t *testing.T) {
	cases := map[string]struct {
		in   string
		def  string
		want string
	}{
		"present": {"value", "default", "value"},
		"empty":   {"", "default", "default"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := fallback(tc.in, tc.def); got != tc.want {
				t.Errorf("fallback(%q, %q) = %q, want %q", tc.in, tc.def, got, tc.want)
			}
		})
	}
}

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
