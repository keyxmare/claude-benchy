// Package report aggregates per-run results into Markdown and HTML documents
// comparing the benchmarked configurations side by side.
package report

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/diffcap"
)

// FileVersion is the final content of a file a config touched, kept so the
// report can show the whole modified file side by side across configs.
type FileVersion struct {
	Path    string
	Content string
}

// Check is the outcome of one deterministic acceptance check run against a
// config's produced workspace.
type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// CriterionEval is the judge's rating of one config against one rubric
// criterion: a level (respecté / partiel / non) and a short justification.
type CriterionEval struct {
	Criterion string `json:"criterion"`
	Level     string `json:"level"`
	Note      string `json:"note,omitempty"`
}

// ConfigEval is the judge's assessment of how well one config met the
// expectation, criterion by criterion — the task itself, not cost or speed.
// Score is derived from the criteria levels (equal weight).
type ConfigEval struct {
	Label    string          `json:"label"`
	Verdict  string          `json:"verdict"`
	Score    int             `json:"score"`
	Criteria []CriterionEval `json:"criteria"`
}

// Evaluation is the cross-config judgement of task fit: an LLM judge rating each
// config against every rubric criterion. Model and Rubric record how the
// judgement was produced.
type Evaluation struct {
	Model          string       `json:"model"`
	Rubric         []string     `json:"rubric"`
	Configs        []ConfigEval `json:"configs"`
	Ranking        []string     `json:"ranking"`
	Recommendation string       `json:"recommendation"`
}

// For returns the judge's assessment of the given config label, or a zero
// ConfigEval when the judge did not rate it.
func (e *Evaluation) For(label string) ConfigEval {
	for _, c := range e.Configs {
		if c.Label == label {
			return c
		}
	}
	return ConfigEval{}
}

// Level returns the config's level for a criterion (matched by its text), or an
// empty string when the judge did not rate it.
func (c ConfigEval) Level(criterion string) string {
	for _, ce := range c.Criteria {
		if ce.Criterion == criterion {
			return ce.Level
		}
	}
	return ""
}

// Note returns the config's observation for a criterion (matched by its text),
// or an empty string when the judge did not rate it.
func (c ConfigEval) Note(criterion string) string {
	for _, ce := range c.Criteria {
		if ce.Criterion == criterion {
			return ce.Note
		}
	}
	return ""
}

// NormalizeLevel maps a free-form judge level onto one of the three canonical
// values: "respecté", "partiel" or "non".
func NormalizeLevel(level string) string {
	s := strings.ToLower(strings.TrimSpace(level))
	switch {
	case strings.HasPrefix(s, "respect"), s == "oui", s == "ok", strings.HasPrefix(s, "met"), s == "full":
		return "respecté"
	case strings.HasPrefix(s, "partiel"), strings.HasPrefix(s, "partial"):
		return "partiel"
	default:
		return "non"
	}
}

// ScoreFromCriteria aggregates criteria levels into a 0-100 score, equal weight
// (respecté = 1, partiel = 0.5, non = 0).
func ScoreFromCriteria(cs []CriterionEval) int {
	if len(cs) == 0 {
		return 0
	}
	sum := 0.0
	for _, c := range cs {
		switch NormalizeLevel(c.Level) {
		case "respecté":
			sum += 1
		case "partiel":
			sum += 0.5
		}
	}
	return int(math.Round(sum / float64(len(cs)) * 100))
}

// CriterionAgg is a criterion's rating for a config aggregated over its runs: a
// consensus level plus a representative observation.
type CriterionAgg struct {
	Criterion string
	Level     string
	Note      string
}

// CheckAgg counts how many of a config's judged runs passed a named check.
type CheckAgg struct {
	Name   string
	Passed int
	Total  int
}

// ConfigScore is a config's task-fit evaluation aggregated over its runs: the
// mean score with its spread, the per-criterion consensus and a representative
// verdict. It derives from the per-run judge ratings, so run-to-run divergence
// is measurable (Min–Max) rather than hidden behind a single number.
type ConfigScore struct {
	Config    string
	Model     string
	Runs      int
	MeanScore int
	MinScore  int
	MaxScore  int
	Verdict   string
	Criteria  []CriterionAgg
	Checks    []CheckAgg
}

// Spread reports whether the config's runs disagreed on the score.
func (c ConfigScore) Spread() bool { return c.MinScore != c.MaxScore }

// LevelOf returns the config's aggregated level for a criterion (matched by its
// text), or an empty string when it was not rated.
func (c ConfigScore) LevelOf(criterion string) string {
	for _, ce := range c.Criteria {
		if ce.Criterion == criterion {
			return ce.Level
		}
	}
	return ""
}

// NoteOf returns the representative observation for a criterion (matched by its
// text), or an empty string when it was not rated.
func (c ConfigScore) NoteOf(criterion string) string {
	for _, ce := range c.Criteria {
		if ce.Criterion == criterion {
			return ce.Note
		}
	}
	return ""
}

// EvalByConfig aggregates the per-run judge ratings into one assessment per
// config, ranked by mean score (descending, stable on ties). It returns nil
// when the bench carried no LLM evaluation. Only OK runs count: a degenerate or
// failed run never drags a config's score down.
func (r Report) EvalByConfig() []ConfigScore {
	if r.Evaluation == nil {
		return nil
	}
	var out []ConfigScore
	for _, g := range r.configGroups() {
		var judged []RunReport
		for _, run := range g.runs {
			if run.OK() && len(r.Evaluation.For(run.Label()).Criteria) > 0 {
				judged = append(judged, run)
			}
		}
		if len(judged) == 0 {
			continue
		}
		out = append(out, r.aggregateConfig(g.config, g.model, judged))
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].MeanScore > out[b].MeanScore })
	return out
}

type configGroup struct {
	config string
	model  string
	runs   []RunReport
}

// configGroups groups the runs by config, preserving first-seen config order
// and run order within each config.
func (r Report) configGroups() []configGroup {
	var groups []configGroup
	index := map[string]int{}
	for _, run := range r.Runs {
		i, ok := index[run.Config]
		if !ok {
			index[run.Config] = len(groups)
			groups = append(groups, configGroup{config: run.Config, model: run.Model})
			i = len(groups) - 1
		}
		groups[i].runs = append(groups[i].runs, run)
	}
	return groups
}

func (r Report) aggregateConfig(config, model string, judged []RunReport) ConfigScore {
	scores := make([]int, len(judged))
	min, max, sum := 100, 0, 0
	for i, run := range judged {
		s := r.Evaluation.For(run.Label()).Score
		scores[i] = s
		sum += s
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}
	mean := int(math.Round(float64(sum) / float64(len(judged))))

	rep := judged[0]
	bestDiff := absInt(scores[0] - mean)
	for i, run := range judged[1:] {
		if d := absInt(scores[i+1] - mean); d < bestDiff {
			rep, bestDiff = run, d
		}
	}
	repEval := r.Evaluation.For(rep.Label())

	cs := ConfigScore{
		Config: config, Model: model, Runs: len(judged),
		MeanScore: mean, MinScore: min, MaxScore: max, Verdict: repEval.Verdict,
	}
	for _, crit := range r.Evaluation.Rubric {
		levels := make([]string, 0, len(judged))
		for _, run := range judged {
			levels = append(levels, r.Evaluation.For(run.Label()).Level(crit))
		}
		cs.Criteria = append(cs.Criteria, CriterionAgg{
			Criterion: crit,
			Level:     aggregateLevel(levels),
			Note:      repEval.Note(crit),
		})
	}
	cs.Checks = aggregateChecks(judged)
	return cs
}

// aggregateLevel buckets the mean of the runs' levels back onto a canonical
// level: mostly respecté → respecté, mostly non → non, else partiel.
func aggregateLevel(levels []string) string {
	if len(levels) == 0 {
		return ""
	}
	sum := 0.0
	for _, l := range levels {
		switch NormalizeLevel(l) {
		case "respecté":
			sum += 1
		case "partiel":
			sum += 0.5
		}
	}
	switch avg := sum / float64(len(levels)); {
	case avg >= 0.75:
		return "respecté"
	case avg >= 0.25:
		return "partiel"
	default:
		return "non"
	}
}

// aggregateChecks counts, per check name, how many of the runs passed it, in
// first-seen order.
func aggregateChecks(runs []RunReport) []CheckAgg {
	var out []CheckAgg
	index := map[string]int{}
	for _, run := range runs {
		for _, c := range run.Checks {
			i, ok := index[c.Name]
			if !ok {
				index[c.Name] = len(out)
				out = append(out, CheckAgg{Name: c.Name})
				i = len(out) - 1
			}
			out[i].Total++
			if c.Passed {
				out[i].Passed++
			}
		}
	}
	return out
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// CheckNames lists the distinct acceptance-check names across all runs, in
// first-seen order — used to surface the evaluation parameters.
func (r Report) CheckNames() []string {
	var names []string
	seen := map[string]bool{}
	for _, run := range r.Runs {
		for _, c := range run.Checks {
			if !seen[c.Name] {
				seen[c.Name] = true
				names = append(names, c.Name)
			}
		}
	}
	return names
}

// RunReport is the outcome of a single config run.
type RunReport struct {
	Config      string
	Run         int
	Model       string
	Metrics     claude.Metrics
	Diff        diffcap.Stats
	Patch       string
	Files       []FileVersion
	Checks      []Check
	ArtifactDir string
	Err         string
}

// ChecksPassed reports how many of the run's checks passed out of the total.
func (r RunReport) ChecksPassed() (passed, total int) {
	for _, c := range r.Checks {
		total++
		if c.Passed {
			passed++
		}
	}
	return passed, total
}

// Label is the human name of a run: the config name, suffixed with the run
// number when a config is benchmarked more than once.
func (r RunReport) Label() string {
	if r.Run > 1 {
		return fmt.Sprintf("%s · run %d", r.Config, r.Run)
	}
	return r.Config
}

// Degenerate reports whether the run technically completed but did no real
// work: it made no tool call, or produced no change at all. Such a run — e.g.
// the model emitting a subagent call as plain text and ending the session — is
// a false success that would otherwise pollute a config's aggregates, so it is
// excluded from them (OK is false) and surfaced distinctly in the report.
func (r RunReport) Degenerate() bool {
	if r.Err != "" || r.Metrics.IsError {
		return false
	}
	return r.Metrics.ToolUses == 0 || r.Diff.FilesChanged == 0
}

// OK reports whether the run succeeded end to end and actually produced work.
func (r RunReport) OK() bool {
	return r.Err == "" && !r.Metrics.IsError && !r.Degenerate()
}

// Report is the aggregate of every run in a benchmark.
type Report struct {
	Prompt      string
	App         string
	GeneratedAt string
	Runs        []RunReport
	// Evaluation is the task-fit judgement, present only when the bench
	// requested an evaluation (rubric and/or checks).
	Evaluation *Evaluation
	// HomeURL, when set, adds a back link at the top of the HTML report. The
	// CLI leaves it empty (a standalone file has nowhere to go); the web
	// dashboard sets it so the report links back to the home page.
	HomeURL string
	// ReuseURL, when set, adds a link that reopens the bench's configuration in
	// the dashboard form for review and re-launch. Set by the web dashboard;
	// empty for CLI reports.
	ReuseURL string
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

	if failed := failedRunLabels(r.Runs); len(failed) > 0 {
		out = append(out, fmt.Sprintf("Runs en échec : %s.", join(failed)))
	}
	if deg := degenerateRunLabels(r.Runs); len(deg) > 0 {
		out = append(out, fmt.Sprintf("Runs sans effet, écartés des classements : %s.", join(deg)))
	}
	return out
}

// Verdict summarises one run's standing across the measured axes.
type Verdict struct {
	Label      string
	Strengths  []string
	Weaknesses []string
}

// Analysis is the cross-config comparison: a verdict per run plus data-driven
// recommendations and evolution leads. It is deterministic and derives only
// from the collected metrics — it judges cost, speed and directness, not the
// intrinsic quality of the produced code.
type Analysis struct {
	Verdicts        []Verdict
	Recommendations []string
}

// axis is one comparable dimension where a lower value is better.
type axis struct {
	name   string
	value  func(RunReport) float64
	format func(float64) string
}

var axes = []axis{
	{"Coût", func(r RunReport) float64 { return r.Metrics.TotalCostUSD }, func(v float64) string { return fmt.Sprintf("$%.4f", v) }},
	{"Durée", func(r RunReport) float64 { return float64(r.Metrics.DurationMS) }, func(v float64) string { return seconds(int(v)) }},
	{"Tours", func(r RunReport) float64 { return float64(r.Metrics.NumTurns) }, func(v float64) string { return fmt.Sprintf("%.0f", v) }},
	{"Appels d'outils", func(r RunReport) float64 { return float64(r.Metrics.ToolUses) }, func(v float64) string { return fmt.Sprintf("%.0f", v) }},
}

// Analysis builds the comparative verdict for every run and a few evolution
// leads. Rankings consider only the runs that succeeded end to end.
func (r Report) Analysis() Analysis {
	var ok []RunReport
	for _, run := range r.Runs {
		if run.OK() {
			ok = append(ok, run)
		}
	}

	verdicts := make([]Verdict, len(r.Runs))
	byLabel := map[string]*Verdict{}
	for i, run := range r.Runs {
		verdicts[i].Label = run.Label()
		byLabel[run.Label()] = &verdicts[i]
	}

	for _, run := range r.Runs {
		switch {
		case run.Degenerate():
			byLabel[run.Label()].Weaknesses = append(byLabel[run.Label()].Weaknesses,
				"Run sans effet : aucun outil appelé ou aucune modification produite")
		case !run.OK():
			byLabel[run.Label()].Weaknesses = append(byLabel[run.Label()].Weaknesses,
				fmt.Sprintf("Run en échec : %s", firstLine(run.Err)))
		}
	}

	if len(ok) >= 2 {
		for _, ax := range axes {
			best := pick(ok, func(a, b RunReport) bool { return ax.value(a) < ax.value(b) })
			worst := pick(ok, func(a, b RunReport) bool { return ax.value(a) > ax.value(b) })
			if ax.value(best) == ax.value(worst) {
				continue
			}
			bv := byLabel[best.Label()]
			bv.Strengths = append(bv.Strengths, fmt.Sprintf("%s au plus bas (%s)", ax.name, ax.format(ax.value(best))))
			wv := byLabel[worst.Label()]
			wv.Weaknesses = append(wv.Weaknesses, fmt.Sprintf("%s au plus haut (%s, %s)",
				ax.name, ax.format(ax.value(worst)), overBest(ax.value(best), ax.value(worst))))
		}

		widest := pick(ok, func(a, b RunReport) bool { return changeSize(a) > changeSize(b) })
		if changeSize(widest) > 0 {
			byLabel[widest.Label()].Strengths = append(byLabel[widest.Label()].Strengths,
				fmt.Sprintf("Périmètre traité le plus large (%d fichier(s), +%d/-%d)",
					widest.Diff.FilesChanged, widest.Diff.Insertions, widest.Diff.Deletions))
		}
		for _, run := range ok {
			if changeSize(run) == 0 {
				byLabel[run.Label()].Weaknesses = append(byLabel[run.Label()].Weaknesses, "Aucune modification produite")
			}
		}
	}

	return Analysis{Verdicts: verdicts, Recommendations: recommend(r, ok)}
}

// recommend derives a handful of evolution leads from the successful runs.
func recommend(r Report, ok []RunReport) []string {
	if len(ok) == 0 {
		return []string{"Aucun run réussi : corriger les échecs avant de comparer les configurations."}
	}

	var out []string

	best := lowestRankSum(ok)
	out = append(out, fmt.Sprintf("Meilleur compromis coût / rapidité / directivité : **%s**.", best.Label()))

	if base, others, gotBase := baseline(ok); gotBase && len(others) > 0 {
		dc, dd, dsz := averageDeltas(base, others)
		out = append(out, fmt.Sprintf(
			"Face à **%s**, les autres configurations coûtent en moyenne %s et sont %s, pour %s.",
			base.Label(), signedPct(dc), slowerFaster(dd), changeDelta(dsz)))
		if dc > 0 && dsz <= 0 {
			out = append(out, fmt.Sprintf(
				"Sur ce prompt, les instructions ajoutées pèsent sans élargir le périmètre traité : envisager de les alléger ou de les évaluer sur un prompt où elles font la différence."))
		}
	}

	if singleRunPerConfig(r) {
		out = append(out, "Un seul run par configuration : les écarts peuvent relever de la variance. Répéter (`runs > 1`) avant de conclure.")
	}

	return out
}

func changeSize(r RunReport) int { return r.Diff.Insertions + r.Diff.Deletions }

// EffCell is one config's standing on one efficiency axis.
type EffCell struct {
	Label string
	Value string
	Level string // respecté (best) | non (worst) | partiel (mid) | "" (no spread)
}

// EffRow is one efficiency axis compared across configs.
type EffRow struct {
	Axis  string
	Cells []EffCell
}

// OKLabels lists the labels of the runs that succeeded, in run order.
func (r Report) OKLabels() []string {
	var out []string
	for _, run := range r.Runs {
		if run.OK() {
			out = append(out, run.Label())
		}
	}
	return out
}

// Efficiency compares the successful runs axis by axis (lower is better),
// marking the best and worst per axis — the aligned, per-axis counterpart of
// the metric verdicts. Returns nil when no run succeeded.
func (r Report) Efficiency() []EffRow {
	ok := okRuns(r.Runs)
	if len(ok) == 0 {
		return nil
	}
	rows := make([]EffRow, 0, len(axes))
	for _, ax := range axes {
		best := pick(ok, func(a, b RunReport) bool { return ax.value(a) < ax.value(b) })
		worst := pick(ok, func(a, b RunReport) bool { return ax.value(a) > ax.value(b) })
		flat := ax.value(best) == ax.value(worst)
		row := EffRow{Axis: ax.name}
		for _, run := range ok {
			cell := EffCell{Label: run.Label(), Value: ax.format(ax.value(run))}
			switch {
			case flat:
				cell.Level = ""
			case ax.value(run) == ax.value(best):
				cell.Level = "respecté"
			case ax.value(run) == ax.value(worst):
				cell.Level = "non"
			default:
				cell.Level = "partiel"
			}
			row.Cells = append(row.Cells, cell)
		}
		rows = append(rows, row)
	}
	return rows
}

func okRuns(runs []RunReport) []RunReport {
	var ok []RunReport
	for _, run := range runs {
		if run.OK() {
			ok = append(ok, run)
		}
	}
	return ok
}

// lowestRankSum returns the run with the best combined rank across every axis.
func lowestRankSum(ok []RunReport) RunReport {
	best := ok[0]
	bestSum := rankSum(ok, ok[0])
	for _, run := range ok[1:] {
		if s := rankSum(ok, run); s < bestSum {
			best, bestSum = run, s
		}
	}
	return best
}

func rankSum(ok []RunReport, run RunReport) int {
	sum := 0
	for _, ax := range axes {
		v := ax.value(run)
		for _, other := range ok {
			if ax.value(other) < v {
				sum++
			}
		}
	}
	return sum
}

// baseline returns the reference run (a config named vanilla or baseline) and
// the others, if such a reference exists among the successful runs.
func baseline(ok []RunReport) (base RunReport, others []RunReport, found bool) {
	idx := -1
	for i, run := range ok {
		if run.Config == "vanilla" || run.Config == "baseline" {
			idx = i
			break
		}
	}
	if idx < 0 {
		return RunReport{}, nil, false
	}
	for i, run := range ok {
		if i != idx {
			others = append(others, run)
		}
	}
	return ok[idx], others, true
}

func averageDeltas(base RunReport, others []RunReport) (cost, duration, size float64) {
	for _, o := range others {
		cost += relDelta(base.Metrics.TotalCostUSD, o.Metrics.TotalCostUSD)
		duration += relDelta(float64(base.Metrics.DurationMS), float64(o.Metrics.DurationMS))
		size += float64(changeSize(o) - changeSize(base))
	}
	n := float64(len(others))
	return cost / n, duration / n, size / n
}

func relDelta(base, v float64) float64 {
	if base == 0 {
		return 0
	}
	return (v - base) / base * 100
}

func singleRunPerConfig(r Report) bool {
	for _, run := range r.Runs {
		if run.Run > 1 {
			return false
		}
	}
	return true
}

// overBest formats how far worst overshoots best, as a percentage.
func overBest(best, worst float64) string {
	if best == 0 {
		return "n/a"
	}
	return fmt.Sprintf("+%.0f %% vs le meilleur", (worst-best)/best*100)
}

func signedPct(v float64) string {
	if v >= 0 {
		return fmt.Sprintf("+%.0f %%", v)
	}
	return fmt.Sprintf("%.0f %%", v)
}

func slowerFaster(deltaPct float64) string {
	if deltaPct >= 0 {
		return fmt.Sprintf("%.0f %% plus lentes", deltaPct)
	}
	return fmt.Sprintf("%.0f %% plus rapides", -deltaPct)
}

func changeDelta(lines float64) string {
	switch {
	case lines > 0:
		return fmt.Sprintf("un périmètre plus large (+%.0f ligne(s) en moyenne)", lines)
	case lines < 0:
		return fmt.Sprintf("un périmètre plus étroit (%.0f ligne(s) en moyenne)", lines)
	default:
		return "un périmètre de changement comparable"
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func pick(runs []RunReport, less func(a, b RunReport) bool) RunReport {
	best := runs[0]
	for _, r := range runs[1:] {
		if less(r, best) {
			best = r
		}
	}
	return best
}

// failedRunLabels lists the runs that hard-failed (a container or Claude error),
// as opposed to those that merely did no work (see degenerateRunLabels).
func failedRunLabels(runs []RunReport) []string {
	var out []string
	for _, r := range runs {
		if r.Err != "" || r.Metrics.IsError {
			out = append(out, r.Label())
		}
	}
	return out
}

// degenerateRunLabels lists the runs that completed without doing any real work
// and are therefore excluded from every ranking.
func degenerateRunLabels(runs []RunReport) []string {
	var out []string
	for _, r := range runs {
		if r.Degenerate() {
			out = append(out, r.Label())
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

// Write renders the report to report.md and report.html under dir.
func Write(dir string, r Report) error {
	md, err := os.Create(filepath.Join(dir, "report.md"))
	if err != nil {
		return err
	}
	defer md.Close()
	if err := WriteMarkdown(md, r); err != nil {
		return err
	}

	html, err := os.Create(filepath.Join(dir, "report.html"))
	if err != nil {
		return err
	}
	defer html.Close()
	return WriteHTML(html, r)
}

// WriteMarkdown renders the report as Markdown.
func WriteMarkdown(w io.Writer, r Report) error {
	return markdownTemplate.Execute(w, r)
}

// WriteHTML renders the report as a self-contained HTML page.
func WriteHTML(w io.Writer, r Report) error {
	return htmlTemplate.Execute(w, r)
}
