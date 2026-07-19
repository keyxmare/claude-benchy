package report_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/diffcap"
	"github.com/keyxmare/claude-benchy/internal/report"
)

func sample() report.Report {
	return report.Report{
		Prompt:      "Add a /health endpoint.",
		App:         "/tmp/app",
		GeneratedAt: "2026-07-15 10:00:00 UTC",
		Runs: []report.RunReport{
			{
				Config: "vanilla", Run: 1, Model: "sonnet",
				ArtifactDir: "vanilla",
				Metrics: claude.Metrics{
					NumTurns: 3, ToolUses: 4, TotalCostUSD: 0.0123,
					DurationMS:    8200,
					ToolBreakdown: map[string]int{"Bash": 2, "Write": 1, "Read": 1},
					Result:        "Ajout de `GET /health`.\n\n**Résumé** : nouvelle route renvoyant\n200 avec un corps JSON minimal. Aucun test existant impacté.",
				},
				Diff:  diffcap.Stats{FilesChanged: 1, Insertions: 3, Deletions: 0},
				Patch: "diff --git a/server.js b/server.js\n@@ -1,2 +1,5 @@\n context\n+added line\n-removed line",
				Files: []report.FileVersion{
					{Path: "server.js", Content: "const app = express();\napp.get('/health', (_, res) => res.json({ ok: true }));\nmodule.exports = app;\n"},
				},
				Checks: []report.Check{
					{Name: "tests", Passed: true},
					{Name: "endpoint présent", Passed: true, Detail: "fichier présent"},
				},
			},
			{
				Config: "strict", Run: 1, Model: "opus",
				ArtifactDir: "strict",
				Metrics: claude.Metrics{
					NumTurns: 6, ToolUses: 9, TotalCostUSD: 0.0456,
					DurationMS:    15400,
					ToolBreakdown: map[string]int{"Edit": 5, "Bash": 2, "Read": 2},
					Result:        "Endpoint ajouté avec validation du statut.",
				},
				Diff:  diffcap.Stats{FilesChanged: 1, Insertions: 5, Deletions: 0},
				Patch: "diff --git a/server.js b/server.js\n@@ -1,2 +1,6 @@\n context\n+more lines",
				Files: []report.FileVersion{
					{Path: "server.js", Content: "const app = express();\napp.get('/health', (req, res) => {\n  res.status(200).json({ ok: true });\n});\nmodule.exports = app;\n"},
				},
				Checks: []report.Check{
					{Name: "tests", Passed: false, Detail: "…FAIL server.test.js"},
					{Name: "endpoint présent", Passed: true, Detail: "fichier présent"},
				},
			},
			{
				Config: "broken", Run: 1, Model: "opus",
				ArtifactDir: "broken",
				Err:         "docker run: exit status 1",
			},
		},
		Evaluation: &report.Evaluation{
			Model:  "sonnet",
			Rubric: []string{"Expose `GET /health` renvoyant 200", "Reste minimal, sans dépendance inutile"},
			Configs: []report.ConfigEval{
				{Label: "vanilla", Score: 100, Verdict: "Répond à l'attendu de façon minimale.",
					Criteria: []report.CriterionEval{
						{Criterion: "Expose `GET /health` renvoyant 200", Level: "respecté", Note: "route et statut corrects"},
						{Criterion: "Reste minimal, sans dépendance inutile", Level: "respecté", Note: "aucune dépendance ajoutée"},
					}},
				{Label: "strict", Score: 50, Verdict: "Correct mais plus verbeux que nécessaire.",
					Criteria: []report.CriterionEval{
						{Criterion: "Expose `GET /health` renvoyant 200", Level: "respecté", Note: "statut HTTP explicite"},
						{Criterion: "Reste minimal, sans dépendance inutile", Level: "non", Note: "code superflu, tests en échec"},
					}},
			},
			Ranking:        []string{"vanilla", "strict"},
			Recommendation: "**vanilla** répond le mieux à l'attendu ici ; réserver les règles aux tâches où la validation compte.",
		},
	}
}

// sampleRepeated exercises the per-config aggregation on the real-world shape:
// several runs per config, one degenerate run (excluded), and diverging scores
// so the report shows a spread and a partially-passing check.
func sampleRepeated() report.Report {
	c1 := "Couvre la logique testable"
	c2 := "Tests idiomatiques (table-driven)"
	okRun := func(cfg string, run, tools, files int, testPass bool) report.RunReport {
		return report.RunReport{
			Config: cfg, Run: run, Model: "sonnet",
			ArtifactDir: cfg + "/run-" + string(rune('0'+run)),
			Metrics:     claude.Metrics{NumTurns: tools + 1, ToolUses: tools, TotalCostUSD: 0.5, DurationMS: 120000, Result: "Tests ajoutés."},
			Diff:        diffcap.Stats{FilesChanged: files, Insertions: 100, Deletions: 0},
			Patch:       "diff --git a/x_test.go b/x_test.go\n@@ -0,0 +1 @@\n+// test",
			Checks: []report.Check{
				{Name: "go test", Passed: testPass, Detail: map[bool]string{true: "", false: "…FAIL"}[testPass]},
				{Name: "go vet", Passed: true},
			},
		}
	}
	crit := func(l1, l2 string) []report.CriterionEval {
		return []report.CriterionEval{{Criterion: c1, Level: l1, Note: "cf. diff"}, {Criterion: c2, Level: l2, Note: "cf. diff"}}
	}
	return report.Report{
		Prompt:      "Écris les tests unitaires manquants.",
		App:         "/tmp/app",
		GeneratedAt: "2026-07-16 22:40:00 CEST",
		Runs: []report.RunReport{
			// Degenerate: the model emitted a subagent call as plain text and ended.
			{Config: "vanilla", Run: 1, Model: "sonnet", ArtifactDir: "vanilla/run-1",
				Metrics: claude.Metrics{NumTurns: 1, ToolUses: 0, TotalCostUSD: 0.07, DurationMS: 8500,
					Result: "Agent({description: \"Survey\", subagent_type: \"Explore\"})"}},
			okRun("vanilla", 2, 20, 3, true),
			okRun("vanilla", 3, 22, 3, false),
			okRun("skill", 1, 18, 2, true),
			okRun("skill", 2, 19, 2, true),
			okRun("skill", 3, 21, 2, true),
		},
		Evaluation: &report.Evaluation{
			Model:  "sonnet",
			Rubric: []string{c1, c2},
			Configs: []report.ConfigEval{
				{Label: "vanilla · run 2", Score: 100, Verdict: "Couverture pure et table-driven.", Criteria: crit("respecté", "respecté")},
				{Label: "vanilla · run 3", Score: 67, Verdict: "Bonne couverture, style peu table-driven.", Criteria: crit("respecté", "partiel")},
				{Label: "skill", Score: 83, Verdict: "Couverture pure, style perfectible.", Criteria: crit("respecté", "partiel")},
				{Label: "skill · run 2", Score: 83, Verdict: "Idem run 1.", Criteria: crit("respecté", "partiel")},
				{Label: "skill · run 3", Score: 83, Verdict: "Idem.", Criteria: crit("respecté", "partiel")},
			},
			Recommendation: "**vanilla** et **skill** convergent ; le run vanilla sans effet est écarté.",
		},
	}
}

func verdict(a report.Analysis, label string) report.Verdict {
	for _, v := range a.Verdicts {
		if v.Label == label {
			return v
		}
	}
	return report.Verdict{}
}

func hasSubstr(items []string, sub string) bool {
	for _, s := range items {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestAnalysisRanksAndRecommends(t *testing.T) {
	a := sample().Analysis()

	if len(a.Verdicts) != 3 {
		t.Fatalf("expected a verdict per run, got %d", len(a.Verdicts))
	}

	// The cheapest, fastest, leanest run collects only strengths.
	vanilla := verdict(a, "vanilla")
	if len(vanilla.Weaknesses) != 0 {
		t.Errorf("vanilla should have no weakness, got %v", vanilla.Weaknesses)
	}
	if !hasSubstr(vanilla.Strengths, "Coût au plus bas") {
		t.Errorf("vanilla should be cheapest, got %v", vanilla.Strengths)
	}

	// The pricier run is flagged, but keeps credit for the widest change set.
	strict := verdict(a, "strict")
	if !hasSubstr(strict.Weaknesses, "Coût au plus haut") {
		t.Errorf("strict should be flagged as most expensive, got %v", strict.Weaknesses)
	}
	if !hasSubstr(strict.Strengths, "Périmètre traité le plus large") {
		t.Errorf("strict should own the widest scope, got %v", strict.Strengths)
	}

	// A failed run is reported as a weakness, never as a strength.
	broken := verdict(a, "broken")
	if len(broken.Strengths) != 0 {
		t.Errorf("failed run should have no strength, got %v", broken.Strengths)
	}
	if !hasSubstr(broken.Weaknesses, "Run en échec") {
		t.Errorf("failed run should be reported, got %v", broken.Weaknesses)
	}

	if !hasSubstr(a.Recommendations, "**vanilla**") {
		t.Errorf("recommendations should name the best compromise, got %v", a.Recommendations)
	}
	if !hasSubstr(a.Recommendations, "variance") {
		t.Errorf("single-run caveat missing, got %v", a.Recommendations)
	}
}

func TestDegenerateAndOK(t *testing.T) {
	cases := map[string]struct {
		run            report.RunReport
		wantOK, wantDe bool
	}{
		"real work":    {report.RunReport{Metrics: claude.Metrics{ToolUses: 3}, Diff: diffcap.Stats{FilesChanged: 1}}, true, false},
		"no tool call": {report.RunReport{Metrics: claude.Metrics{ToolUses: 0}, Diff: diffcap.Stats{FilesChanged: 1}}, false, true},
		"empty diff":   {report.RunReport{Metrics: claude.Metrics{ToolUses: 5}, Diff: diffcap.Stats{FilesChanged: 0}}, false, true},
		"hard error":   {report.RunReport{Err: "docker run: boom"}, false, false},
		"claude error": {report.RunReport{Metrics: claude.Metrics{IsError: true}}, false, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.run.OK(); got != tc.wantOK {
				t.Errorf("OK() = %v, want %v", got, tc.wantOK)
			}
			if got := tc.run.Degenerate(); got != tc.wantDe {
				t.Errorf("Degenerate() = %v, want %v", got, tc.wantDe)
			}
		})
	}
}

func TestEvalByConfigAggregatesRunsAndExcludesDegenerate(t *testing.T) {
	crit := []string{"c1", "c2"}
	okRun := func(cfg string, run, tools, files int) report.RunReport {
		return report.RunReport{Config: cfg, Run: run, Model: "sonnet",
			Metrics: claude.Metrics{ToolUses: tools}, Diff: diffcap.Stats{FilesChanged: files},
			Checks: []report.Check{{Name: "go test", Passed: true}}}
	}
	r := report.Report{
		Runs: []report.RunReport{
			// run 1 is degenerate (no tool call, no diff): must be excluded.
			{Config: "vanilla", Run: 1, Model: "sonnet"},
			okRun("vanilla", 2, 5, 2),
			okRun("vanilla", 3, 4, 1),
			okRun("strict", 1, 6, 1),
		},
		Evaluation: &report.Evaluation{
			Rubric: crit,
			Configs: []report.ConfigEval{
				{Label: "vanilla · run 2", Score: 100, Verdict: "run 2 verdict", Criteria: []report.CriterionEval{
					{Criterion: "c1", Level: "respecté", Note: "n1"}, {Criterion: "c2", Level: "respecté", Note: "n2"}}},
				{Label: "vanilla · run 3", Score: 50, Verdict: "run 3 verdict", Criteria: []report.CriterionEval{
					{Criterion: "c1", Level: "respecté", Note: "n1b"}, {Criterion: "c2", Level: "non", Note: "n2b"}}},
				{Label: "strict", Score: 40, Verdict: "strict verdict", Criteria: []report.CriterionEval{
					{Criterion: "c1", Level: "partiel", Note: "s1"}, {Criterion: "c2", Level: "non", Note: "s2"}}},
			},
		},
	}

	got := r.EvalByConfig()
	if len(got) != 2 {
		t.Fatalf("expected 2 configs, got %d: %+v", len(got), got)
	}

	// Ranked by mean score: vanilla (round((100+50)/2)=75) before strict (40).
	v := got[0]
	if v.Config != "vanilla" {
		t.Fatalf("expected vanilla ranked first, got %q", got[0].Config)
	}
	if v.Runs != 2 {
		t.Errorf("degenerate run should be excluded, judged runs = %d, want 2", v.Runs)
	}
	if v.MeanScore != 75 || v.MinScore != 50 || v.MaxScore != 100 {
		t.Errorf("mean/spread wrong: mean=%d min=%d max=%d", v.MeanScore, v.MinScore, v.MaxScore)
	}
	if !v.Spread() {
		t.Error("expected Spread() true for diverging runs")
	}
	// Representative run (closest to mean, ties → first) is run 2.
	if v.Verdict != "run 2 verdict" {
		t.Errorf("representative verdict = %q, want run 2's", v.Verdict)
	}
	// c1 respecté in both runs → respecté; c2 respecté+non → partiel.
	if v.Criteria[0].Level != "respecté" || v.Criteria[1].Level != "partiel" {
		t.Errorf("aggregated levels wrong: %+v", v.Criteria)
	}
	if len(v.Checks) != 1 || v.Checks[0].Name != "go test" || v.Checks[0].Passed != 2 || v.Checks[0].Total != 2 {
		t.Errorf("check aggregation wrong: %+v", v.Checks)
	}

	if got[1].Config != "strict" || got[1].Spread() {
		t.Errorf("single-run config should have no spread: %+v", got[1])
	}
}

func TestEvalByConfigNilWithoutEvaluation(t *testing.T) {
	if got := (report.Report{Runs: []report.RunReport{{Config: "a"}}}).EvalByConfig(); got != nil {
		t.Errorf("expected nil without an evaluation, got %+v", got)
	}
}

func TestAnalysisWithNoSuccessfulRun(t *testing.T) {
	r := report.Report{Runs: []report.RunReport{{Config: "x", Err: "boom"}}}
	a := r.Analysis()
	if !hasSubstr(a.Recommendations, "Aucun run réussi") {
		t.Errorf("expected a no-success recommendation, got %v", a.Recommendations)
	}
}

func TestGolden(t *testing.T) {
	cases := []struct {
		name  string
		write func(w *bytes.Buffer) error
	}{
		{"report.md", func(w *bytes.Buffer) error { return report.WriteMarkdown(w, sample()) }},
		{"report.html", func(w *bytes.Buffer) error { return report.WriteHTML(w, sample()) }},
		{"report-repeated.md", func(w *bytes.Buffer) error { return report.WriteMarkdown(w, sampleRepeated()) }},
		{"report-repeated.html", func(w *bytes.Buffer) error { return report.WriteHTML(w, sampleRepeated()) }},
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

func TestApplyButtonOnlyWhenServed(t *testing.T) {
	served := sample()
	served.TranscriptBase = "benches/x/results/ts"
	served.Runs[0].Bundle = "/some/bundle"
	var withServer bytes.Buffer
	if err := report.WriteHTML(&withServer, served); err != nil {
		t.Fatal(err)
	}
	body := withServer.String()
	if !strings.Contains(body, `action="/apply"`) {
		t.Error("served report should offer an apply form")
	}
	if !strings.Contains(body, `name="artifact" value="vanilla"`) {
		t.Error("apply form should carry the run's artifact dir")
	}
	if !strings.Contains(body, `class="export-config"`) || !strings.Contains(body, `action="/export-config"`) {
		t.Error("served report should offer config export for a run with a known bundle")
	}

	var standalone bytes.Buffer
	if err := report.WriteHTML(&standalone, sample()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(standalone.String(), `action="/apply"`) {
		t.Error("standalone report (no server) must not offer an apply form")
	}
	if strings.Contains(standalone.String(), `action="/export-config"`) {
		t.Error("standalone report must not offer config export")
	}
}

func TestNormalizeLevel(t *testing.T) {
	for in, want := range map[string]report.Level{
		"respecté":           report.Met,
		"respectée à moitié": report.Met,
		"OUI":                report.Met,
		"ok":                 report.Met,
		"met the bar":        report.Met,
		"full":               report.Met,
		"partiellement":      report.Partial,
		"partial credit":     report.Partial,
		"non":                report.Unmet,
		"n'importe quoi":     report.Unmet,
		"":                   report.Unrated,
		"   ":                report.Unrated,
	} {
		if got := report.NormalizeLevel(in); got != want {
			t.Errorf("report.NormalizeLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScoreFromCriteria(t *testing.T) {
	crit := func(levels ...string) []report.CriterionEval {
		cs := make([]report.CriterionEval, len(levels))
		for i, l := range levels {
			cs[i] = report.CriterionEval{Level: l}
		}
		return cs
	}
	tests := []struct {
		name string
		in   []report.CriterionEval
		want int
	}{
		{"empty", nil, 0},
		{"all met", crit("respecté", "respecté"), 100},
		{"met and unmet", crit("respecté", "non"), 50},
		{"met partial unmet rounds", crit("respecté", "partiel", "non"), 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := report.ScoreFromCriteria(tt.in); got != tt.want {
				t.Errorf("report.ScoreFromCriteria(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestLevelWeight(t *testing.T) {
	for level, want := range map[report.Level]float64{report.Met: 1, report.Partial: 0.5, report.Unmet: 0, report.Unrated: 0} {
		if got := level.Weight(); got != want {
			t.Errorf("report.Level(%q).Weight() = %v, want %v", level, got, want)
		}
	}
}

func TestLevelCSSClass(t *testing.T) {
	for level, want := range map[report.Level]string{report.Met: "ok", report.Partial: "warn", report.Unmet: "ko", report.Unrated: "na"} {
		if got := level.CSSClass(); got != want {
			t.Errorf("report.Level(%q).CSSClass() = %q, want %q", level, got, want)
		}
	}
}

func TestLevelSymbol(t *testing.T) {
	for level, want := range map[report.Level]string{report.Met: "✓", report.Partial: "~", report.Unmet: "✗", report.Unrated: "–"} {
		if got := level.Symbol(); got != want {
			t.Errorf("report.Level(%q).Symbol() = %q, want %q", level, got, want)
		}
	}
}

func TestEfficiencyRanksBestMidWorst(t *testing.T) {
	run := func(cost float64) report.RunReport {
		return report.RunReport{
			Metrics: claude.Metrics{ToolUses: 1, NumTurns: 1, DurationMS: 1000, TotalCostUSD: cost},
			Diff:    diffcap.Stats{FilesChanged: 1},
		}
	}
	r := report.Report{Runs: []report.RunReport{run(0.01), run(0.02), run(0.03)}}

	var cost report.EffRow
	for _, row := range r.Efficiency() {
		if row.Axis == "Coût" {
			cost = row
		}
	}

	want := []report.Rank{report.Best, report.Mid, report.Worst}
	var got []report.Rank
	for _, c := range cost.Cells {
		got = append(got, c.Rank)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Efficiency() Coût ranks mismatch (-want +got):\n%s", diff)
	}
}

func TestEfficiencyNoSuccessfulRun(t *testing.T) {
	if got := (report.Report{Runs: []report.RunReport{{Err: "boom"}}}).Efficiency(); got != nil {
		t.Errorf("Efficiency() with no OK run = %v, want nil", got)
	}
}

func TestRankCSSClass(t *testing.T) {
	for rank, want := range map[report.Rank]string{report.Best: "ok", report.Mid: "warn", report.Worst: "ko", report.None: "na"} {
		if got := rank.CSSClass(); got != want {
			t.Errorf("report.Rank(%q).CSSClass() = %q, want %q", rank, got, want)
		}
	}
}

func TestRankSymbol(t *testing.T) {
	for rank, want := range map[report.Rank]string{report.Best: "✓", report.Mid: "~", report.Worst: "✗", report.None: "–"} {
		if got := rank.Symbol(); got != want {
			t.Errorf("report.Rank(%q).Symbol() = %q, want %q", rank, got, want)
		}
	}
}

func TestEvaluationForUnknownLabel(t *testing.T) {
	e := &report.Evaluation{Configs: []report.ConfigEval{{Label: "vanilla", Score: 80}}}

	got := e.For("missing")

	if diff := cmp.Diff(report.ConfigEval{}, got); diff != "" {
		t.Errorf("For(unknown) should be the zero ConfigEval (-want +got):\n%s", diff)
	}
}

func TestConfigEvalLevelAndNoteUnknownCriterion(t *testing.T) {
	c := report.ConfigEval{Criteria: []report.CriterionEval{{Criterion: "c1", Level: "respecté", Note: "ok"}}}

	if got := c.Level("absent"); got != "" {
		t.Errorf("Level(unknown) = %q, want empty", got)
	}
	if got := c.Note("absent"); got != "" {
		t.Errorf("Note(unknown) = %q, want empty", got)
	}
}

func TestConfigScoreLevelOfAndNoteOfUnknownCriterion(t *testing.T) {
	c := report.ConfigScore{Criteria: []report.CriterionAgg{{Criterion: "c1", Level: "respecté", Note: "ok"}}}

	if got := c.LevelOf("absent"); got != "" {
		t.Errorf("LevelOf(unknown) = %q, want empty", got)
	}
	if got := c.NoteOf("absent"); got != "" {
		t.Errorf("NoteOf(unknown) = %q, want empty", got)
	}
}

func TestSynthesisNoSuccessfulRun(t *testing.T) {
	r := report.Report{Runs: []report.RunReport{{Config: "x", Err: "boom"}}}

	got := r.Synthesis()

	want := []string{"Aucun run réussi : voir les erreurs par config."}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Synthesis() with no OK run mismatch (-want +got):\n%s", diff)
	}
}

// TestEvalByConfigRepresentativeClosestToMean pins the representative-run choice
// to the run whose score is nearest the mean when it is not the first run.
func TestEvalByConfigRepresentativeClosestToMean(t *testing.T) {
	okRun := func(run int) report.RunReport {
		return report.RunReport{Config: "c", Run: run, Model: "sonnet",
			Metrics: claude.Metrics{ToolUses: 3}, Diff: diffcap.Stats{FilesChanged: 1}}
	}
	r := report.Report{
		Runs: []report.RunReport{okRun(1), okRun(2), okRun(3)},
		Evaluation: &report.Evaluation{
			Rubric: []string{"c1"},
			Configs: []report.ConfigEval{
				{Label: "c", Score: 100, Verdict: "run 1", Criteria: []report.CriterionEval{{Criterion: "c1", Level: "respecté"}}},
				{Label: "c · run 2", Score: 0, Verdict: "run 2", Criteria: []report.CriterionEval{{Criterion: "c1", Level: "non"}}},
				{Label: "c · run 3", Score: 50, Verdict: "run 3", Criteria: []report.CriterionEval{{Criterion: "c1", Level: "partiel"}}},
			},
		},
	}

	got := r.EvalByConfig()

	if len(got) != 1 {
		t.Fatalf("expected 1 config, got %d", len(got))
	}
	// mean = round((100+0+50)/3) = 50; run 3 (score 50) is closest, not run 1.
	if got[0].Verdict != "run 3" {
		t.Errorf("representative verdict = %q, want run 3's", got[0].Verdict)
	}
}

// TestAnalysisFlagsSuccessfulRunWithoutChanges covers an OK run that touched a
// file but produced no line change (changeSize 0): it earns the "no change"
// weakness and never the widest-scope strength.
func TestAnalysisFlagsSuccessfulRunWithoutChanges(t *testing.T) {
	noChange := func(cfg string, cost float64, turns int) report.RunReport {
		return report.RunReport{Config: cfg, Run: 1, Model: "sonnet",
			Metrics: claude.Metrics{ToolUses: 2, NumTurns: turns, DurationMS: 1000, TotalCostUSD: cost},
			Diff:    diffcap.Stats{FilesChanged: 1, Insertions: 0, Deletions: 0}}
	}
	r := report.Report{Runs: []report.RunReport{noChange("a", 0.01, 2), noChange("b", 0.02, 3)}}

	a := r.Analysis()

	for _, label := range []string{"a", "b"} {
		v := verdict(a, label)
		if !hasSubstr(v.Weaknesses, "Aucune modification produite") {
			t.Errorf("%s should be flagged for producing no change, got %v", label, v.Weaknesses)
		}
		if hasSubstr(v.Strengths, "Périmètre traité le plus large") {
			t.Errorf("%s should not own widest scope when nothing changed, got %v", label, v.Strengths)
		}
	}
}

// TestRecommendBaselineHeavierWithoutWiderScope covers the lead that fires when
// the non-baseline configs cost more (dc > 0) without widening the change set
// (dsz <= 0). It also exercises the "baseline"-named reference branch.
func TestRecommendBaselineHeavierWithoutWiderScope(t *testing.T) {
	base := report.RunReport{Config: "baseline", Run: 1, Model: "sonnet",
		Metrics: claude.Metrics{ToolUses: 3, NumTurns: 2, DurationMS: 1000, TotalCostUSD: 0.01},
		Diff:    diffcap.Stats{FilesChanged: 1, Insertions: 100, Deletions: 0}}
	other := report.RunReport{Config: "strict", Run: 1, Model: "sonnet",
		Metrics: claude.Metrics{ToolUses: 4, NumTurns: 3, DurationMS: 2000, TotalCostUSD: 0.02},
		Diff:    diffcap.Stats{FilesChanged: 1, Insertions: 10, Deletions: 0}}
	r := report.Report{Runs: []report.RunReport{base, other}}

	recs := r.Analysis().Recommendations

	if !hasSubstr(recs, "Face à **baseline**") {
		t.Errorf("expected a baseline comparison lead, got %v", recs)
	}
	if !hasSubstr(recs, "les instructions ajoutées pèsent") {
		t.Errorf("expected the heavier-without-wider lead, got %v", recs)
	}
}

func TestRecommendWithoutBaseline(t *testing.T) {
	run := func(cfg string, cost float64, turns int) report.RunReport {
		return report.RunReport{Config: cfg, Run: 1, Model: "sonnet",
			Metrics: claude.Metrics{ToolUses: 2, NumTurns: turns, DurationMS: 1000, TotalCostUSD: cost},
			Diff:    diffcap.Stats{FilesChanged: 1, Insertions: 5, Deletions: 0}}
	}
	r := report.Report{Runs: []report.RunReport{run("alpha", 0.01, 2), run("beta", 0.02, 3)}}

	recs := r.Analysis().Recommendations

	if hasSubstr(recs, "Face à") {
		t.Errorf("no baseline config: no baseline comparison expected, got %v", recs)
	}
	if !hasSubstr(recs, "Meilleur compromis") {
		t.Errorf("expected the best-compromise lead, got %v", recs)
	}
}

// TestWrite covers the happy path and both os.Create error returns. The
// remaining branch — WriteMarkdown returning an error — is unreachable here: it
// executes a template that invokes no error-returning func or method over a
// freshly created *os.File, so it can only fail on a write error a temp file
// never produces. It stays justified, not tested.
func TestWrite(t *testing.T) {
	t.Run("writes both documents", func(t *testing.T) {
		dir := t.TempDir()

		if err := report.Write(dir, sample()); err != nil {
			t.Fatalf("Write() = %v, want nil", err)
		}

		for _, name := range []string{"report.md", "report.html"} {
			b, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("reading %s: %v", name, err)
			}
			if len(b) == 0 {
				t.Errorf("%s is empty", name)
			}
		}
	})

	t.Run("markdown create error on a missing directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "does-not-exist")

		if err := report.Write(dir, sample()); err == nil {
			t.Error("Write() to a nonexistent directory = nil, want error")
		}
	})

	t.Run("html create error", func(t *testing.T) {
		dir := t.TempDir()
		// A directory named report.html makes os.Create fail after report.md
		// was written, exercising the second create's error path.
		if err := os.Mkdir(filepath.Join(dir, "report.html"), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := report.Write(dir, sample()); err == nil {
			t.Error("Write() with report.html as a directory = nil, want error")
		}
	})
}
