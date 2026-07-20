package report

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	htmltmpl "html/template"
	"net/url"
	"path"
	"sort"
	"strings"
	texttmpl "text/template"
)

// Static assets of the report, kept in their own files so the editor and the
// linters see real CSS/JS; they are inlined into the self-contained HTML report
// at generation (see htmlFuncs), never linked.
var (
	//go:embed static/report.css
	reportCSS string
	//go:embed static/compare.js
	compareJS string
	//go:embed static/export.js
	exportJS string
	//go:embed static/theme.js
	themeJS string
	//go:embed static/theme-init.js
	themeInitJS string
)

func styleCSS() htmltmpl.CSS       { return htmltmpl.CSS(reportCSS) }
func compareScript() htmltmpl.JS   { return htmltmpl.JS(compareJS) }
func exportScript() htmltmpl.JS    { return htmltmpl.JS(exportJS) }
func themeScript() htmltmpl.JS     { return htmltmpl.JS(themeJS) }
func themeInitScript() htmltmpl.JS { return htmltmpl.JS(themeInitJS) }

// toolSummary formats a run's tool usage, most-used first, e.g.
// "Edit ×12 · Read ×8 · Bash ×5". Empty when the run recorded no tool breakdown.
func toolSummary(r RunReport) string {
	b := r.Metrics.ToolBreakdown
	if len(b) == 0 {
		return ""
	}
	type stat struct {
		name string
		n    int
	}
	stats := make([]stat, 0, len(b))
	for name, n := range b {
		stats = append(stats, stat{name, n})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].n != stats[j].n {
			return stats[i].n > stats[j].n
		}
		return stats[i].name < stats[j].name
	})
	parts := make([]string, len(stats))
	for i, s := range stats {
		parts[i] = fmt.Sprintf("%s ×%d", s.name, s.n)
	}
	return strings.Join(parts, " · ")
}

// fileChange is one path touched by a run and how it changed.
type fileChange struct {
	Path   string
	Status string // "added", "removed" or "modified"
}

// parseChanges extracts the per-file change status from a unified git diff.
func parseChanges(patch string) []fileChange {
	var out []fileChange
	lines := strings.Split(patch, "\n")
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "diff --git ") {
			continue
		}
		fc := fileChange{Path: diffGitPath(lines[i]), Status: "modified"}
		for j := i + 1; j < len(lines) && !strings.HasPrefix(lines[j], "diff --git "); j++ {
			switch {
			case strings.HasPrefix(lines[j], "new file mode"):
				fc.Status = "added"
			case strings.HasPrefix(lines[j], "deleted file mode"):
				fc.Status = "removed"
			}
		}
		if fc.Path != "" {
			out = append(out, fc)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// diffGitPath returns the destination path of a `diff --git a/X b/X` line.
func diffGitPath(line string) string {
	if i := strings.Index(line, " b/"); i >= 0 {
		return line[i+3:]
	}
	return ""
}

// changeList exposes a run's file changes to the Markdown template.
func changeList(r RunReport) []fileChange { return parseChanges(r.Patch) }

// changeSym is the Markdown marker for a change status.
func changeSym(status string) string {
	switch status {
	case "added":
		return "+"
	case "removed":
		return "-"
	default:
		return "~"
	}
}

type treeNode struct {
	name     string
	status   string
	children map[string]*treeNode
}

func buildTree(changes []fileChange) *treeNode {
	root := &treeNode{children: map[string]*treeNode{}}
	for _, c := range changes {
		cur := root
		parts := strings.Split(c.Path, "/")
		for k, p := range parts {
			child, ok := cur.children[p]
			if !ok {
				child = &treeNode{name: p, children: map[string]*treeNode{}}
				cur.children[p] = child
			}
			if k == len(parts)-1 {
				child.status = c.Status
			}
			cur = child
		}
	}
	return root
}

// fileTreeHTML renders a run's touched files as a nested tree, coloured by
// change status, for the side-by-side comparison's global overview.
func fileTreeHTML(r RunReport) htmltmpl.HTML {
	changes := parseChanges(r.Patch)
	if len(changes) == 0 {
		return htmltmpl.HTML(`<p class="empty">Aucune modification.</p>`)
	}
	var b strings.Builder
	renderTree(&b, buildTree(changes))
	return htmltmpl.HTML(b.String())
}

func renderTree(b *strings.Builder, n *treeNode) {
	names := make([]string, 0, len(n.children))
	for name := range n.children {
		names = append(names, name)
	}
	// Directories first, then files, each alphabetical.
	sort.Slice(names, func(i, j int) bool {
		di := len(n.children[names[i]].children) > 0
		dj := len(n.children[names[j]].children) > 0
		if di != dj {
			return di
		}
		return names[i] < names[j]
	})
	b.WriteString(`<ul class="ftree">`)
	for _, name := range names {
		c := n.children[name]
		if len(c.children) > 0 {
			b.WriteString(`<li class="dir">` + html.EscapeString(name) + "/")
			renderTree(b, c)
			b.WriteString("</li>")
		} else {
			b.WriteString(`<li class="file ` + c.status + `">` + html.EscapeString(name) + "</li>")
		}
	}
	b.WriteString("</ul>")
}

// transcriptHref builds the dashboard URL replaying a run's transcript, from the
// output directory base and the run's artifact subdirectory.
func transcriptHref(base, artifact string) string {
	return "/transcript?dir=" + url.QueryEscape(path.Join(base, artifact))
}

func status(r RunReport) string {
	switch {
	case r.Err != "":
		return "erreur"
	case r.Metrics.IsError:
		return "erreur-claude"
	case r.Degenerate():
		return "sans effet"
	default:
		return "ok"
	}
}

// winnerConfig is the name of the highest-scoring config, or empty when no
// evaluation ran. EvalByConfig sorts by mean score descending, so the first
// entry wins; the HTML report highlights that config's rows.
func winnerConfig(r Report) string {
	scores := r.EvalByConfig()
	if len(scores) == 0 {
		return ""
	}
	return scores[0].Config
}

func statusClass(r RunReport) string {
	switch {
	case r.OK():
		return "ok"
	case r.Degenerate():
		return "warn"
	default:
		return "err"
	}
}

// checkClass maps an aggregated check (how many runs passed) onto a CSS class:
// all passed → ok, none → ko, some → warn.
func checkClass(c CheckAgg) string {
	switch {
	case c.Total > 0 && c.Passed == c.Total:
		return "ok"
	case c.Passed == 0:
		return "ko"
	default:
		return "warn"
	}
}

func cost(r RunReport) string {
	return fmt.Sprintf("$%.4f", r.Metrics.TotalCostUSD)
}

func seconds(ms int) string {
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

// checks renders the passed/total ratio of a run's acceptance checks, or a dash
// when the bench declared none.
func checks(r RunReport) string {
	passed, total := r.ChecksPassed()
	if total == 0 {
		return "—"
	}
	return fmt.Sprintf("%d/%d", passed, total)
}

func add(a, b int) int { return a + b }

// levelClass maps a free-form judge criterion level onto a CSS class: ok / warn
// / ko, or na when the criterion was not rated.
func levelClass(level string) string {
	return NormalizeLevel(level).CSSClass()
}

// mdCell makes a string safe inside a Markdown table cell: pipes escaped,
// newlines flattened to spaces.
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "|", "\\|")
}

// levelSymbol is the glyph shown for a level in the criteria matrix.
func levelSymbol(level string) string {
	return NormalizeLevel(level).Symbol()
}

// inlineHTML renders the inline Markdown subset (**bold**, `code`) inside an
// already HTML-escaped string.
func inlineHTML(escaped string) string {
	var b strings.Builder
	code, bold := false, false
	for i := 0; i < len(escaped); i++ {
		switch {
		case escaped[i] == '`':
			if code {
				b.WriteString("</code>")
			} else {
				b.WriteString("<code>")
			}
			code = !code
		case !code && i+1 < len(escaped) && escaped[i] == '*' && escaped[i+1] == '*':
			if bold {
				b.WriteString("</strong>")
			} else {
				b.WriteString("<strong>")
			}
			bold = !bold
			i++
		default:
			b.WriteByte(escaped[i])
		}
	}
	if code {
		b.WriteString("</code>")
	}
	if bold {
		b.WriteString("</strong>")
	}
	return b.String()
}

// markdownInline turns the small subset of Markdown used in bullets
// (**bold**, `code`) into safe HTML.
func markdownInline(s string) htmltmpl.HTML {
	return htmltmpl.HTML(inlineHTML(html.EscapeString(s)))
}

// resultHTML renders Claude's result summary as block Markdown: blank lines
// separate paragraphs, "- "/"* " lines become bullet lists, and inline
// **bold**/`code` is honoured. Soft line wraps inside a paragraph collapse to a
// single space so wrapped prose no longer renders with a break on every line.
func resultHTML(text string) htmltmpl.HTML {
	var b strings.Builder
	for _, block := range splitBlocks(strings.TrimSpace(text)) {
		lines := strings.Split(block, "\n")
		if isBulletList(lines) {
			b.WriteString("<ul>")
			for _, line := range lines {
				item := strings.TrimSpace(line)[2:]
				b.WriteString("<li>")
				b.WriteString(inlineHTML(html.EscapeString(item)))
				b.WriteString("</li>")
			}
			b.WriteString("</ul>")
			continue
		}
		b.WriteString("<p>")
		b.WriteString(inlineHTML(html.EscapeString(joinWrapped(lines))))
		b.WriteString("</p>")
	}
	return htmltmpl.HTML(b.String())
}

func splitBlocks(s string) []string {
	var blocks []string
	var cur []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			if len(cur) > 0 {
				blocks = append(blocks, strings.Join(cur, "\n"))
				cur = nil
			}
			continue
		}
		cur = append(cur, line)
	}
	if len(cur) > 0 {
		blocks = append(blocks, strings.Join(cur, "\n"))
	}
	return blocks
}

func isBulletList(lines []string) bool {
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "- ") && !strings.HasPrefix(t, "* ") {
			return false
		}
	}
	return len(lines) > 0
}

func joinWrapped(lines []string) string {
	trimmed := make([]string, len(lines))
	for i, l := range lines {
		trimmed[i] = strings.TrimSpace(l)
	}
	return strings.Join(trimmed, " ")
}

// filesJSON serialises every touched file per run so the side-by-side view can
// diff any two configurations in the browser. json.Marshal escapes <, > and &,
// so the payload is safe to embed verbatim inside a <script> element.
func filesJSON(r Report) htmltmpl.JS {
	type payload struct {
		Labels []string                     `json:"labels"`
		Files  map[string]map[string]string `json:"files"`
	}
	p := payload{Files: map[string]map[string]string{}}
	for _, run := range r.Runs {
		if len(run.Files) == 0 {
			continue
		}
		label := run.Label()
		p.Labels = append(p.Labels, label)
		files := map[string]string{}
		for _, f := range run.Files {
			files[f.Path] = f.Content
		}
		p.Files[label] = files
	}
	b, err := json.Marshal(p)
	if err != nil {
		return htmltmpl.JS(`{"labels":[],"files":{}}`)
	}
	return htmltmpl.JS(b)
}

var funcs = map[string]any{
	"transcriptHref": transcriptHref,
	"status":         status,
	"statusClass":    statusClass,
	"cost":           cost,
	"seconds":        seconds,
	"checks":         checks,
	"add":            add,
	"levelClass":     levelClass,
	"levelSymbol":    levelSymbol,
	"checkClass":     checkClass,
	"mdCell":         mdCell,
	"toolSummary":    toolSummary,
	"changeList":     changeList,
	"changeSym":      changeSym,
}

const markdownSource = `# Rapport claude-benchy

- **Généré :** {{.GeneratedAt}}
- **App :** {{.App}}

## Prompt

` + "```" + `
{{.Prompt}}
` + "```" + `

## Synthèse
{{range .Synthesis}}
- {{.}}
{{- end}}

## Comparaison

| Config | Modèle | Statut | Checks | Tours | Outils | Coût | Durée | Fichiers | +Lignes | -Lignes |
|--------|--------|--------|--------|-------|--------|------|-------|----------|---------|---------|
{{- range .Runs}}
| {{.Label}} | {{.Model}} | {{status .}} | {{checks .}} | {{.Metrics.NumTurns}} | {{.Metrics.ToolUses}} | {{cost .}} | {{seconds .Metrics.DurationMS}} | {{.Diff.FilesChanged}} | {{.Diff.Insertions}} | {{.Diff.Deletions}} |
{{- end}}
{{if .Evaluation}}
## Évaluation de l'attendu

Notée **critère par critère** par **{{.Evaluation.Model}}** (sur le fond, pas le coût). Paramètres définis dans ` + "`bench.yaml`" + ` → ` + "`evaluate`" + `.

Critères (rubric) :
{{range .Evaluation.Rubric}}
- {{.}}
{{- end}}
{{- if .CheckNames}}

Vérifications : {{range $i, $n := .CheckNames}}{{if $i}} · {{end}}{{$n}}{{end}}
{{- end}}

Chaque config est notée sur ses runs exploitables (les runs sans effet ou en échec sont écartés). Le score est la **moyenne** des runs, la dispersion min–max mesure la divergence.

### Matrice critères × config
{{$byconf := .EvalByConfig}}
| Critère |{{range $byconf}} {{.Config}} ({{.Runs}} run(s)) |{{end}}
|---|{{range $byconf}}---|{{end}}
{{- range $crit := .Evaluation.Rubric}}
| {{$crit}} |{{range $c := $byconf}} {{levelSymbol ($c.LevelOf $crit)}} {{mdCell ($c.NoteOf $crit)}} |{{end}}
{{- end}}
| **Score moyen** |{{range $c := $byconf}} {{$c.MeanScore}}/100{{if $c.Spread}} ({{$c.MinScore}}–{{$c.MaxScore}}){{end}} |{{end}}
{{range $c := $byconf}}
### {{$c.Config}} — {{$c.MeanScore}}/100{{if $c.Spread}} (dispersion {{$c.MinScore}}–{{$c.MaxScore}}){{end}} · {{$c.Runs}} run(s)
{{if $c.Verdict}}
{{$c.Verdict}}
{{- end}}
{{- if $c.Checks}}

Vérifications automatiques (runs réussis) :
{{range $c.Checks}}
- [{{if eq .Passed .Total}}x{{else}} {{end}}] {{.Name}} — {{.Passed}}/{{.Total}} run(s)
{{- end}}
{{- end}}
{{end}}
**Recommandation** : {{.Evaluation.Recommendation}}
{{end}}
## Efficacité (coût & rapidité)

Chaque axe comparé entre configs (✓ meilleur, ✗ moins bon).
{{if .Efficiency}}
| Axe |{{range .OKLabels}} {{.}} |{{end}}
|---|{{range .OKLabels}}---|{{end}}
{{- range .Efficiency}}
| {{.Axis}} |{{range .Cells}} {{if .Rank}}{{.Rank.Symbol}} {{end}}{{.Value}} |{{end}}
{{- end}}
{{end}}
### Recommandations
{{range .Analysis.Recommendations}}
- {{.}}
{{- end}}

## Arbres de fichiers

Fichiers touchés par chaque configuration (+ ajouté, ~ modifié, - supprimé).
{{range .Runs}}
### {{.Label}}
{{if changeList .}}
{{range changeList .}}- {{changeSym .Status}} {{.Path}}
{{end}}
{{- else}}
_Aucune modification._
{{end}}
{{- end}}

## Détail par config
{{range .Runs}}
### {{.Label}} — {{status .}}

- **Artefacts :** {{.ArtifactDir}}
{{- if .Err}}
- **Erreur :** {{.Err}}
{{- end}}
{{- if .Metrics.Result}}

**Résultat**

{{.Metrics.Result}}
{{- end}}
{{- if toolSummary .}}

**Outils utilisés :** {{toolSummary .}}
{{- end}}
{{- if .Checks}}

**Vérifications**
{{range .Checks}}
- {{if .Passed}}✓{{else}}✗{{end}} {{.Name}}{{with .Detail}} — {{. | mdCell}}{{end}}
{{- end}}
{{- end}}

_Code produit : voir la comparaison côte à côte._
{{end}}`

const htmlSource = `<!doctype html>
<html lang="fr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Rapport claude-benchy</title>
<script>{{themeInitJS}}</script>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;500;600;700&family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
<style>
{{styleCSS}}</style>
</head>
<body>
<header class="topbar">
  {{if .HomeURL}}<a class="brand" href="{{.HomeURL}}"><span class="logo">b</span><span class="name">benchy</span></a>{{else}}<span class="brand"><span class="logo">b</span><span class="name">benchy</span></span>{{end}}
  <span class="root">rapport</span>
  <span class="spacer"></span>
  {{if .HomeURL}}<nav class="nav-seg" aria-label="Sections">
    <a class="seg" href="{{.HomeURL}}">Dashboard</a>
    <span class="seg disabled" aria-disabled="true">Run live</span>
    <span class="seg active" aria-current="page">Rapport</span>
  </nav>{{end}}
  <button type="button" id="theme-toggle" class="theme-toggle" title="Basculer le thème">☀</button>
</header>
<div class="page">
{{if .ReuseURL}}<p class="backlink"><a href="{{.ReuseURL}}">← Reprendre cette config</a></p>
{{end}}<p class="eyebrow">Rapport · {{.GeneratedAt}}</p>
<h1>Rapport claude-benchy</h1>
<p class="meta-list"><strong>Généré :</strong> {{.GeneratedAt}} &nbsp;·&nbsp; <strong>App :</strong> <code>{{.App}}</code></p>

<div class="layout">
<main>

<div class="grid-2 tight">
<section>
<h2 id="prompt">Prompt</h2>
<div class="prompt">{{.Prompt}}</div>
</section>
<section>
<h2 id="synthese">Synthèse</h2>
<ul class="synthesis">
{{- range .Synthesis}}
<li>{{. | markdownInline}}</li>
{{- end}}
</ul>
</section>
</div>

<h2 id="comparaison">Comparaison</h2>
{{- $winner := winnerConfig .}}
<table>
<thead>
<tr><th>Config</th><th>Modèle</th><th>Statut</th><th>Checks</th><th>Tours</th><th>Outils</th><th>Coût</th><th>Durée</th><th>Fichiers</th><th>+Lignes</th><th>-Lignes</th>{{if .TranscriptBase}}<th>Détail</th>{{end}}</tr>
</thead>
<tbody>
{{- range .Runs}}
<tr{{if not .OK}} class="row-degenerate"{{else if and $winner (eq .Config $winner)}} class="row-winner"{{end}}>
<td>{{.Label}}</td>
<td>{{.Model}}</td>
<td><span class="badge {{statusClass .}}">{{status .}}</span></td>
<td class="num">{{checks .}}</td>
<td class="num">{{.Metrics.NumTurns}}</td>
<td class="num">{{.Metrics.ToolUses}}</td>
<td class="num">{{cost .}}</td>
<td class="num">{{seconds .Metrics.DurationMS}}</td>
<td class="num">{{.Diff.FilesChanged}}</td>
<td class="num">{{.Diff.Insertions}}</td>
<td class="num">{{.Diff.Deletions}}</td>
{{- if $.TranscriptBase}}<td><a href="{{transcriptHref $.TranscriptBase .ArtifactDir}}">transcript</a></td>{{end}}
</tr>
{{- end}}
</tbody>
</table>

{{if .Evaluation}}
{{- $byconf := .EvalByConfig}}
<h2 id="evaluation">Évaluation de l'attendu</h2>
<p class="meta-list">Chaque <strong>config</strong> est notée <strong>critère par critère</strong> par un juge, sur le fond (pas sur le coût). Le score agrège ses runs exploitables : moyenne, et dispersion min–max qui mesure la divergence entre runs. Les runs sans effet ou en échec sont écartés.</p>

<div class="synthesis params">
<strong>Paramètres d'évaluation</strong>
<dl>
<dt>Juge</dt><dd><code>{{.Evaluation.Model}}</code></dd>
<dt>Critères (rubric)</dt><dd><ol>{{range .Evaluation.Rubric}}<li>{{.}}</li>{{end}}</ol></dd>
{{- if .CheckNames}}
<dt>Vérifications</dt><dd>{{range $i, $n := .CheckNames}}{{if $i}} · {{end}}{{$n}}{{end}}</dd>
{{- end}}
</dl>
<p class="params-note">Définis dans <code>bench.yaml</code> → <code>evaluate.rubric</code> / <code>evaluate.checks</code> / <code>evaluate.model</code>.</p>
</div>

{{- if .Evaluation.Rubric}}
<div class="sxs-scroll">
<table class="matrix">
<thead>
<tr><th>Critère</th>{{range $byconf}}<th>{{.Config}} <span class="runs">{{.Runs}} run(s)</span></th>{{end}}</tr>
</thead>
<tbody>
{{- range $crit := .Evaluation.Rubric}}
<tr>
<td class="crit">{{$crit}}</td>
{{- range $c := $byconf}}
{{- $lvl := $c.LevelOf $crit}}
<td class="lvl {{levelClass $lvl}}"><span class="mark" title="{{$lvl}}">{{levelSymbol $lvl}}</span>{{with $c.NoteOf $crit}}<span class="obs">{{. | markdownInline}}</span>{{end}}</td>
{{- end}}
</tr>
{{- end}}
</tbody>
<tfoot>
<tr><td class="crit">Score moyen</td>{{range $c := $byconf}}<td class="num scorerow">{{$c.MeanScore}}/100{{if $c.Spread}} <span class="spread">({{$c.MinScore}}–{{$c.MaxScore}})</span>{{end}}</td>{{end}}</tr>
</tfoot>
</table>
</div>
{{- end}}

<div class="verdicts">
{{- range $c := $byconf}}
<div class="verdict">
<h3>{{$c.Config}} <span class="score">{{$c.MeanScore}}/100</span></h3>
{{- if $c.Spread}}
<p class="verdict-spread">{{$c.Runs}} runs · dispersion {{$c.MinScore}}–{{$c.MaxScore}}</p>
{{- else if gt $c.Runs 1}}
<p class="verdict-spread">{{$c.Runs}} runs · score stable</p>
{{- end}}
{{- if $c.Verdict}}
<p class="verdict-line">{{$c.Verdict | markdownInline}}</p>
{{- end}}
{{- if $c.Checks}}
<ul class="checks">
{{- range $c.Checks}}
<li class="{{checkClass .}}">{{.Name}} <span class="stat">{{.Passed}}/{{.Total}} run(s)</span></li>
{{- end}}
</ul>
{{- end}}
</div>
{{- end}}
</div>
<div class="synthesis reco">
<strong>Recommandation</strong>
<p>{{.Evaluation.Recommendation | markdownInline}}</p>
</div>
{{end}}

<h2 id="efficacite">Efficacité (coût &amp; rapidité)</h2>
<p class="meta-list">Chaque axe comparé entre configs : <span class="mark ok-fg">✓</span> le meilleur, <span class="mark ko-fg">✗</span> le moins bon.</p>
{{- if .Efficiency}}
<div class="sxs-scroll">
<table class="matrix">
<thead>
<tr><th>Axe</th>{{range .OKLabels}}<th>{{.}}</th>{{end}}</tr>
</thead>
<tbody>
{{- range .Efficiency}}
<tr>
<td class="crit">{{.Axis}}</td>
{{- range .Cells}}
<td class="lvl {{.Rank.CSSClass}}">{{if .Rank}}<span class="mark">{{.Rank.Symbol}}</span>{{end}}<span class="obs">{{.Value}}</span></td>
{{- end}}
</tr>
{{- end}}
</tbody>
</table>
</div>
{{- end}}
<div class="synthesis reco">
<strong>Recommandations</strong>
<ul>
{{- range .Analysis.Recommendations}}
<li>{{. | markdownInline}}</li>
{{- end}}
</ul>
</div>

<h2 id="arbres">Arbres de fichiers</h2>
<p class="meta-list ftree-legend">Vue d'ensemble des fichiers touchés par chaque configuration :
<span class="added">+ ajouté</span> · ~ modifié · <span class="removed">− supprimé</span>.</p>
<div class="ftrees">
{{- range .Runs}}
<div class="ftree-card">
<h3>{{.Label}}</h3>
{{fileTreeHTML .}}
</div>
{{- end}}
</div>

<h2 id="cote-a-cote">Comparaison côte à côte</h2>
<p class="meta-list">Code complet des fichiers touchés, deux configurations côte à côte. Une ligne présente d'un seul côté (un ajout de cette config) est surlignée en vert ; une ligne qui diffère entre les deux est surlignée en ambre.</p>
<div class="sxs-controls">
<label>Config gauche <select id="sxs-left"></select></label>
<label>Config droite <select id="sxs-right"></select></label>
</div>
<div id="sxs-body"></div>

<h2 id="detail">Détail par config</h2>
<p class="meta-list">Résultat, vérifications déterministes et application des modifications. Le code produit se consulte dans la <a href="#cote-a-cote">comparaison côte à côte</a>.</p>
{{range .Runs}}
<div class="card">
<header>
<h3>{{.Label}}</h3>
<span class="badge {{statusClass .}}">{{status .}}</span>
<span class="spacer"></span>
<span class="stat">{{seconds .Metrics.DurationMS}} · {{cost .}} · {{.Metrics.NumTurns}} tours · +{{.Diff.Insertions}}/-{{.Diff.Deletions}}</span>
</header>
<div class="body">
<div class="card-info">
<p class="meta-list">Artefacts : <code>{{.ArtifactDir}}</code></p>
{{- if .Err}}
<p class="badge err">Erreur : {{.Err}}</p>
{{- end}}
{{- if .Metrics.Result}}
<div class="result">{{.Metrics.Result | resultHTML}}</div>
{{- end}}
{{- if toolSummary .}}
<h4 class="detail-sub">Outils utilisés</h4>
<p class="meta-list tool-summary">{{toolSummary .}}</p>
{{- end}}
{{- if .Checks}}
<h4 class="detail-sub">Vérifications</h4>
<ul class="checklist">
{{- range .Checks}}
<li class="{{if .Passed}}ok{{else}}ko{{end}}">{{.Name}}{{with .Detail}} <span class="stat">{{.}}</span>{{end}}</li>
{{- end}}
</ul>
{{- end}}
{{- if and $.TranscriptBase (gt .Diff.FilesChanged 0)}}
<form class="apply" method="post" action="/apply" onsubmit="return confirm('Appliquer {{.Diff.FilesChanged}} fichier(s) modifié(s) par « {{.Label}} » sur le projet testé ? Les changements ne seront pas committés.');">
<input type="hidden" name="dir" value="{{$.TranscriptBase}}">
<input type="hidden" name="artifact" value="{{.ArtifactDir}}">
<button type="submit" class="primary">Appliquer au projet ({{.Diff.FilesChanged}} fichier(s))</button>
</form>
{{- end}}
{{- if and $.TranscriptBase .Bundle}}
<p><button type="button" class="export-config" data-artifact="{{.ArtifactDir}}" data-label="{{.Label}}">Exporter la conf…</button></p>
{{- end}}
</div>
</div>
</div>
{{end}}

</main>
<aside class="toc">
<div class="toc-inner">
<p class="toc-title">Sur cette page</p>
<nav id="toc-nav">
<a href="#prompt">Prompt</a>
<a href="#synthese">Synthèse</a>
<a href="#comparaison">Comparaison</a>
{{if .Evaluation}}<a href="#evaluation">Évaluation de l'attendu</a>{{end}}
<a href="#efficacite">Efficacité</a>
<a href="#arbres">Arbres de fichiers</a>
<a href="#cote-a-cote">Comparaison côte à côte</a>
<a href="#detail">Détail par config</a>
</nav>
</div>
</aside>
</div>
</div>

<script>{{themeJS}}</script>
<script>
var BENCHY_FILES = {{filesJSON .}};
{{compareJS}}</script>

{{if .TranscriptBase}}
<dialog id="cfg-export" class="picker">
<form method="post" action="/export-config" id="cfg-form">
<div class="picker-head">
<strong id="cfg-title" class="picker-path">Exporter la conf</strong>
<button type="button" id="cfg-close" aria-label="Fermer">×</button>
</div>
<input type="hidden" name="dir" value="{{.TranscriptBase}}">
<input type="hidden" name="artifact" id="cfg-artifact">
<label class="cfg-all"><input type="checkbox" id="cfg-selectall" checked> Tout sélectionner</label>
<ul id="cfg-files" class="picker-list"></ul>
<div class="picker-actions">
<button type="submit" class="primary">Exporter la sélection</button>
</div>
</form>
</dialog>
<script>
{{exportJS}}</script>
{{end}}
</body>
</html>
`

var (
	markdownTemplate = texttmpl.Must(texttmpl.New("md").Funcs(funcs).Parse(markdownSource))
	htmlTemplate     = htmltmpl.Must(htmltmpl.New("html").Funcs(htmlFuncs()).Parse(htmlSource))
)

func htmlFuncs() htmltmpl.FuncMap {
	fm := htmltmpl.FuncMap{}
	for k, v := range funcs {
		fm[k] = v
	}
	fm["resultHTML"] = resultHTML
	fm["markdownInline"] = markdownInline
	fm["filesJSON"] = filesJSON
	fm["fileTreeHTML"] = fileTreeHTML
	fm["styleCSS"] = styleCSS
	fm["compareJS"] = compareScript
	fm["exportJS"] = exportScript
	fm["themeJS"] = themeScript
	fm["themeInitJS"] = themeInitScript
	fm["winnerConfig"] = winnerConfig
	return fm
}
