package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/docker"
	"github.com/keyxmare/claude-benchy/internal/report"
	"github.com/keyxmare/claude-benchy/internal/spec"
)

// maxDiffBytes caps each diff embedded in the judge prompt so the evaluation
// call stays token-bounded regardless of how large a change a config produced.
const maxDiffBytes = 6000

// judgeInput is the data fed to the fixed judge prompt template.
type judgeInput struct {
	Prompt  string
	Rubric  []string
	Configs []judgeConfig
}

type judgeConfig struct {
	Label  string
	Status string
	Checks string
	Result string
	Diff   string
}

// judgeSchema mirrors the JSON the judge is asked to return: a level per rubric
// criterion, per config. Score and ranking are derived deterministically from
// the levels afterwards, not left to the model.
type judgeSchema struct {
	Configs []struct {
		Label    string                 `json:"label"`
		Verdict  string                 `json:"verdict"`
		Criteria []report.CriterionEval `json:"criteria"`
	} `json:"configs"`
	Recommendation string `json:"recommendation"`
}

const judgePromptSource = `Tu es un évaluateur logiciel impartial. Plusieurs configurations de Claude Code ont traité LA MÊME tâche sur LE MÊME dépôt. Note chaque configuration STRICTEMENT au regard des critères ci-dessous (le rubric) — pas sur le coût ni la vitesse.

## Tâche demandée
{{.Prompt}}

## Critères d'évaluation (rubric)
{{range .Rubric}}- {{.}}
{{end}}
## Productions à évaluer
{{range .Configs}}
### {{.Label}} (statut : {{.Status}}{{if .Checks}}, vérifications : {{.Checks}}{{end}})
Résumé de l'agent : {{.Result}}
Diff produit :
` + "```" + `
{{.Diff}}
` + "```" + `
{{end}}
## Réponse attendue
Pour CHAQUE configuration, note CHAQUE critère du rubric (dans le même ordre) par un niveau parmi "respecté", "partiel" ou "non", avec une justification courte et factuelle. Réponds UNIQUEMENT par un objet JSON valide, sans texte ni balise Markdown autour, de la forme :
{"configs":[{"label":"<nom exact fourni>","verdict":"<une phrase de synthèse>","criteria":[{"criterion":"<texte exact du critère>","level":"respecté|partiel|non","note":"<justification courte>"}]}],"recommendation":"<1 à 3 phrases, dont une piste d'évolution si pertinent>"}
Utilise exactement les labels fournis, et un objet criteria par critère du rubric, dans l'ordre.`

var judgeTemplate = template.Must(template.New("judge").Parse(judgePromptSource))

// judge asks an LLM to score how well each config met the expectation. It
// returns nil-safe errors: a judge failure never aborts a benchmark, it just
// leaves the report without an evaluation.
func judge(ctx context.Context, s *spec.Spec, rep report.Report, outputRoot string, opts Options) (*report.Evaluation, error) {
	prompt, err := buildJudgePrompt(rep, s.Evaluate.Rubric)
	if err != nil {
		return nil, err
	}

	// Models occasionally emit slightly malformed JSON (e.g. a dropped brace).
	// One corrective retry recovers these transient cases.
	var parsed report.Evaluation
	var parseErr error
	for attempt := 0; attempt < 2; attempt++ {
		p := prompt
		if attempt > 0 {
			p += "\n\nATTENTION : ta réponse précédente n'était pas un JSON valide. Renvoie EXACTEMENT un seul objet JSON conforme au schéma ci-dessus, sans aucun texte ni balise autour."
		}
		answer, err := runJudge(ctx, s, p, outputRoot, opts)
		if err != nil {
			return nil, err
		}
		if parsed, parseErr = parseJudge(answer); parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		return nil, parseErr
	}
	parsed.Model = s.Evaluate.Model
	parsed.Rubric = s.Evaluate.Rubric
	return &parsed, nil
}

func buildJudgePrompt(rep report.Report, rubric []string) (string, error) {
	in := judgeInput{Prompt: strings.TrimSpace(rep.Prompt), Rubric: rubric}
	for _, run := range rep.Runs {
		// Skip failed and degenerate runs: judging a no-op wastes a judge call
		// and would inject a bogus 0/100 that drags the config's mean down.
		if !run.OK() {
			continue
		}
		in.Configs = append(in.Configs, judgeConfig{
			Label:  run.Label(),
			Status: statusLabel(run),
			Checks: checksSummary(run),
			Result: fallback(strings.TrimSpace(run.Metrics.Result), "(aucun résumé)"),
			Diff:   fallback(truncate(run.Patch, maxDiffBytes), "(aucune modification)"),
		})
	}
	var buf bytes.Buffer
	if err := judgeTemplate.Execute(&buf, in); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func runJudge(ctx context.Context, s *spec.Spec, prompt, outputRoot string, opts Options) (string, error) {
	scratch := filepath.Join(outputRoot, ".judge")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(scratch) }()

	var transcript bytes.Buffer
	runSpec := docker.RunSpec{
		Image:     opts.Image,
		WorkDir:   scratch,
		CredsFile: s.CredsFile,
		Env:       map[string]string{"DISABLE_AUTOUPDATER": "1"},
		Args: []string{
			"-p", prompt,
			"--output-format", "stream-json",
			"--verbose",
			"--dangerously-skip-permissions",
			"--model", s.Evaluate.Model,
		},
	}
	runErr := opts.Docker.Run(ctx, runSpec, &transcript, &bytes.Buffer{})
	_ = os.WriteFile(filepath.Join(outputRoot, "judge-transcript.jsonl"), transcript.Bytes(), 0o644)
	if runErr != nil {
		return "", fmt.Errorf("judge run: %w", runErr)
	}

	metrics, err := claude.Parse(bytes.NewReader(transcript.Bytes()))
	if err != nil {
		return "", fmt.Errorf("parse judge transcript: %w", err)
	}
	if metrics.IsError {
		return "", fmt.Errorf("judge reported an error: %s", metrics.Subtype)
	}
	return metrics.Result, nil
}

// parseJudge extracts the JSON object from the judge's answer (tolerating code
// fences or surrounding prose) and decodes it.
func parseJudge(answer string) (report.Evaluation, error) {
	raw := extractJSONObject(answer)
	if raw == "" {
		return report.Evaluation{}, fmt.Errorf("no JSON object in judge answer")
	}
	var out judgeSchema
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return report.Evaluation{}, fmt.Errorf("decode judge JSON: %w", err)
	}
	ev := report.Evaluation{Recommendation: out.Recommendation}
	for _, c := range out.Configs {
		ce := report.ConfigEval{Label: c.Label, Verdict: c.Verdict, Criteria: c.Criteria}
		ce.Score = report.ScoreFromCriteria(ce.Criteria)
		ev.Configs = append(ev.Configs, ce)
	}
	ev.Ranking = rankByScore(ev.Configs)
	return ev, nil
}

// rankByScore orders config labels by descending score, stable on ties.
func rankByScore(cs []report.ConfigEval) []string {
	order := make([]int, len(cs))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return cs[order[a]].Score > cs[order[b]].Score })
	out := make([]string, len(cs))
	for i, j := range order {
		out[i] = cs[j].Label
	}
	return out
}

func extractJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end < start {
		return ""
	}
	return s[start : end+1]
}

func statusLabel(r report.RunReport) string {
	if r.OK() {
		return "réussi"
	}
	return "échec"
}

func checksSummary(r report.RunReport) string {
	passed, total := r.ChecksPassed()
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", passed, total)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n… (diff tronqué)"
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
