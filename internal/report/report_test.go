package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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
				Config: "vanilla", Run: 1, Model: "sonnet",
				ArtifactDir: "vanilla",
				Metrics: claude.Metrics{
					NumTurns: 3, ToolUses: 4, TotalCostUSD: 0.0123,
					DurationMS: 8200,
					Result:     "Ajout de `GET /health`.\n\n**Résumé** : nouvelle route renvoyant\n200 avec un corps JSON minimal. Aucun test existant impacté.",
				},
				Diff:  diffcap.Stats{FilesChanged: 1, Insertions: 3, Deletions: 0},
				Patch: "diff --git a/server.js b/server.js\n@@ -1,2 +1,5 @@\n context\n+added line\n-removed line",
				Files: []FileVersion{
					{Path: "server.js", Content: "const app = express();\napp.get('/health', (_, res) => res.json({ ok: true }));\nmodule.exports = app;\n"},
				},
				Checks: []Check{
					{Name: "tests", Passed: true},
					{Name: "endpoint présent", Passed: true, Detail: "fichier présent"},
				},
			},
			{
				Config: "strict", Run: 1, Model: "opus",
				ArtifactDir: "strict",
				Metrics: claude.Metrics{
					NumTurns: 6, ToolUses: 9, TotalCostUSD: 0.0456,
					DurationMS: 15400,
					Result:     "Endpoint ajouté avec validation du statut.",
				},
				Diff:  diffcap.Stats{FilesChanged: 1, Insertions: 5, Deletions: 0},
				Patch: "diff --git a/server.js b/server.js\n@@ -1,2 +1,6 @@\n context\n+more lines",
				Files: []FileVersion{
					{Path: "server.js", Content: "const app = express();\napp.get('/health', (req, res) => {\n  res.status(200).json({ ok: true });\n});\nmodule.exports = app;\n"},
				},
				Checks: []Check{
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
		Evaluation: &Evaluation{
			Model:  "sonnet",
			Rubric: []string{"Expose `GET /health` renvoyant 200", "Reste minimal, sans dépendance inutile"},
			Configs: []ConfigEval{
				{Label: "vanilla", Score: 100, Verdict: "Répond à l'attendu de façon minimale.",
					Criteria: []CriterionEval{
						{Criterion: "Expose `GET /health` renvoyant 200", Level: "respecté", Note: "route et statut corrects"},
						{Criterion: "Reste minimal, sans dépendance inutile", Level: "respecté", Note: "aucune dépendance ajoutée"},
					}},
				{Label: "strict", Score: 50, Verdict: "Correct mais plus verbeux que nécessaire.",
					Criteria: []CriterionEval{
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
func sampleRepeated() Report {
	c1 := "Couvre la logique testable"
	c2 := "Tests idiomatiques (table-driven)"
	okRun := func(cfg string, run, tools, files int, testPass bool) RunReport {
		return RunReport{
			Config: cfg, Run: run, Model: "sonnet",
			ArtifactDir: cfg + "/run-" + string(rune('0'+run)),
			Metrics:     claude.Metrics{NumTurns: tools + 1, ToolUses: tools, TotalCostUSD: 0.5, DurationMS: 120000, Result: "Tests ajoutés."},
			Diff:        diffcap.Stats{FilesChanged: files, Insertions: 100, Deletions: 0},
			Patch:       "diff --git a/x_test.go b/x_test.go\n@@ -0,0 +1 @@\n+// test",
			Checks: []Check{
				{Name: "go test", Passed: testPass, Detail: map[bool]string{true: "", false: "…FAIL"}[testPass]},
				{Name: "go vet", Passed: true},
			},
		}
	}
	crit := func(l1, l2 string) []CriterionEval {
		return []CriterionEval{{Criterion: c1, Level: l1, Note: "cf. diff"}, {Criterion: c2, Level: l2, Note: "cf. diff"}}
	}
	return Report{
		Prompt:      "Écris les tests unitaires manquants.",
		App:         "/tmp/app",
		GeneratedAt: "2026-07-16 22:40:00 CEST",
		Runs: []RunReport{
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
		Evaluation: &Evaluation{
			Model:  "sonnet",
			Rubric: []string{c1, c2},
			Configs: []ConfigEval{
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

func verdict(a Analysis, label string) Verdict {
	for _, v := range a.Verdicts {
		if v.Label == label {
			return v
		}
	}
	return Verdict{}
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
		run            RunReport
		wantOK, wantDe bool
	}{
		"real work":    {RunReport{Metrics: claude.Metrics{ToolUses: 3}, Diff: diffcap.Stats{FilesChanged: 1}}, true, false},
		"no tool call": {RunReport{Metrics: claude.Metrics{ToolUses: 0}, Diff: diffcap.Stats{FilesChanged: 1}}, false, true},
		"empty diff":   {RunReport{Metrics: claude.Metrics{ToolUses: 5}, Diff: diffcap.Stats{FilesChanged: 0}}, false, true},
		"hard error":   {RunReport{Err: "docker run: boom"}, false, false},
		"claude error": {RunReport{Metrics: claude.Metrics{IsError: true}}, false, false},
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
	okRun := func(cfg string, run, tools, files int) RunReport {
		return RunReport{Config: cfg, Run: run, Model: "sonnet",
			Metrics: claude.Metrics{ToolUses: tools}, Diff: diffcap.Stats{FilesChanged: files},
			Checks: []Check{{Name: "go test", Passed: true}}}
	}
	r := Report{
		Runs: []RunReport{
			// run 1 is degenerate (no tool call, no diff): must be excluded.
			{Config: "vanilla", Run: 1, Model: "sonnet"},
			okRun("vanilla", 2, 5, 2),
			okRun("vanilla", 3, 4, 1),
			okRun("strict", 1, 6, 1),
		},
		Evaluation: &Evaluation{
			Rubric: crit,
			Configs: []ConfigEval{
				{Label: "vanilla · run 2", Score: 100, Verdict: "run 2 verdict", Criteria: []CriterionEval{
					{Criterion: "c1", Level: "respecté", Note: "n1"}, {Criterion: "c2", Level: "respecté", Note: "n2"}}},
				{Label: "vanilla · run 3", Score: 50, Verdict: "run 3 verdict", Criteria: []CriterionEval{
					{Criterion: "c1", Level: "respecté", Note: "n1b"}, {Criterion: "c2", Level: "non", Note: "n2b"}}},
				{Label: "strict", Score: 40, Verdict: "strict verdict", Criteria: []CriterionEval{
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
	if got := (Report{Runs: []RunReport{{Config: "a"}}}).EvalByConfig(); got != nil {
		t.Errorf("expected nil without an evaluation, got %+v", got)
	}
}

func TestAnalysisWithNoSuccessfulRun(t *testing.T) {
	r := Report{Runs: []RunReport{{Config: "x", Err: "boom"}}}
	a := r.Analysis()
	if !hasSubstr(a.Recommendations, "Aucun run réussi") {
		t.Errorf("expected a no-success recommendation, got %v", a.Recommendations)
	}
}

func TestResultHTMLCollapsesSoftWraps(t *testing.T) {
	got := string(resultHTML("Ligne un\nligne deux.\n\n**Gras** et `code`."))
	want := "<p>Ligne un ligne deux.</p><p><strong>Gras</strong> et <code>code</code>.</p>"
	if got != want {
		t.Errorf("resultHTML() =\n%s\nwant\n%s", got, want)
	}
}

func TestGolden(t *testing.T) {
	cases := []struct {
		name  string
		write func(w *bytes.Buffer) error
	}{
		{"report.md", func(w *bytes.Buffer) error { return WriteMarkdown(w, sample()) }},
		{"report.html", func(w *bytes.Buffer) error { return WriteHTML(w, sample()) }},
		{"report-repeated.md", func(w *bytes.Buffer) error { return WriteMarkdown(w, sampleRepeated()) }},
		{"report-repeated.html", func(w *bytes.Buffer) error { return WriteHTML(w, sampleRepeated()) }},
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
