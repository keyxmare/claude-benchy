package report

import (
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
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Montserrat:ital,wght@0,400;0,500;0,600;0,700;0,800;1,700;1,800&family=Inter:wght@400;500;600&display=swap" rel="stylesheet">
<style>
/* Système de design Gazoline (Motoblouz) : noir/blanc purs, accent jaune
   #f1ab00 employé avec parcimonie, Montserrat, radius anguleux, pas d'ombres.
   Contraintes de contraste respectées : le jaune ne sert jamais de texte sur
   fond clair (yellow-800 ou noir), bordures de contrôle en grey-700. */
:root {
  --mb-yellow-100: #fbe6b3; --mb-yellow-500: #f1ab00; --mb-yellow-600: #d99a00;
  --mb-yellow-700: #c18900; --mb-yellow-800: #a97800;
  --mb-grey-100: #fbfbfb; --mb-grey-200: #f6f6f6; --mb-grey-300: #ececec;
  --mb-grey-400: #d4d4d4; --mb-grey-500: #bdbdbd; --mb-grey-700: #6f6e6e;
  --mb-grey-800: #5e5e5e; --mb-grey-900: #2f2f2f;

  --bg: #ffffff; --surface: #ffffff; --surface-quiet: var(--mb-grey-200);
  --ink: #000000; --ink-quiet: var(--mb-grey-700);
  --border: var(--mb-grey-400); --border-strong: var(--mb-grey-700);
  --accent: var(--mb-yellow-500); --accent-ink: #000000; --accent-text: var(--mb-yellow-800);
  --brand-bg: #000000; --brand-ink: #ffffff;
  --ok: #0f7000; --err: #d70321; --warn: var(--mb-yellow-800); --focus: var(--mb-yellow-500);
  --add-bg: #eaf6ea; --add-fg: #0f6b00; --del-bg: #fdeaec; --del-fg: #a1021a; --hunk: #31708a;
  --chg-bg: #fbe6b3; --chg-fg: #7a5800;

  --r-card: 16px; --r-ctl: 8px; --r-full: 999px;
  --font-display: 'Montserrat', 'Segoe UI', system-ui, -apple-system, sans-serif;
  --font-body: 'Montserrat', 'Segoe UI', system-ui, -apple-system, sans-serif;
  --font-ui: 'Inter', 'Montserrat', system-ui, sans-serif;
  --font-mono: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  color-scheme: light;
}
* { box-sizing: border-box; }
body { font-family: var(--font-body); margin: 0; max-width: none;
  padding: 2.5rem clamp(1.25rem, 3vw, 3rem) 4rem;
  background: var(--bg); color: var(--ink); line-height: 1.55; font-size: 16px;
  -webkit-font-smoothing: antialiased; }
.topbar { display: flex; align-items: baseline; gap: 1rem;
  margin: -2.5rem calc(-1 * clamp(1.25rem, 3vw, 3rem)) 2rem;
  padding: 0.9rem clamp(1.25rem, 3vw, 3rem);
  background: var(--brand-bg); color: var(--brand-ink); }
.topbar .brand { font-family: var(--font-display); font-weight: 800; letter-spacing: 0.02em;
  text-transform: lowercase; font-size: 1.2rem; }
.topbar .brand a { color: inherit; text-decoration: none; }
.topbar .root { color: color-mix(in srgb, var(--brand-ink) 60%, transparent);
  font-size: 0.8rem; font-family: var(--font-mono); }
h1 { font-family: var(--font-display); font-weight: 800; font-style: italic; text-transform: uppercase;
  letter-spacing: -0.01em; line-height: 1.02; font-size: clamp(2.2rem, 6vw, 3.4rem);
  margin: 0 0 0.3rem; text-wrap: balance; }
h1::after { content: ""; display: block; width: 3.5rem; height: 6px; background: var(--accent);
  margin-top: 0.7rem; border-radius: 2px; }
h2 { font-family: var(--font-display); font-weight: 700; letter-spacing: -0.01em; font-size: 1.55rem;
  margin: 3rem 0 0.6rem; padding-bottom: 0.4rem; border-bottom: 2px solid var(--ink); text-wrap: balance; }
h3 { font-family: var(--font-display); font-weight: 600; }
a { color: var(--accent-text); text-underline-offset: 2px; }
.meta-list { color: var(--ink-quiet); font-size: 0.9rem; }
.backlink { margin: 0 0 1rem; font-family: var(--font-ui); font-size: 0.85rem; }
.backlink a { text-decoration: none; font-weight: 600; }
code { font-family: var(--font-mono); background: var(--surface-quiet);
  padding: 0.08rem 0.35rem; border-radius: 4px; font-size: 0.88em; }
.prompt { background: var(--brand-bg); color: var(--brand-ink); border-left: 6px solid var(--accent);
  border-radius: var(--r-card); padding: 1rem 1.2rem; white-space: pre-wrap; font-size: 0.95rem; }
.synthesis { background: var(--surface-quiet); border: 1px solid var(--border);
  border-left: 6px solid var(--accent); border-radius: var(--r-card); padding: 0.7rem 1.4rem; }
.synthesis li { margin: 0.3rem 0; }
table { border-collapse: collapse; width: 100%; margin: 1rem 0; font-family: var(--font-ui); font-size: 0.92rem; }
th, td { border: 1px solid var(--border); padding: 0.5rem 0.75rem; text-align: left; }
th { background: var(--ink); color: var(--bg); font-family: var(--font-display); font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.03em; font-size: 0.76rem; }
td.num { text-align: right; font-variant-numeric: tabular-nums; }
.badge { font-family: var(--font-display); font-weight: 700; padding: 0.12rem 0.6rem;
  border-radius: var(--r-full); font-size: 0.78rem; }
.badge.ok { color: var(--ok); background: color-mix(in srgb, var(--ok) 16%, transparent); }
.badge.err { color: var(--err); background: color-mix(in srgb, var(--err) 16%, transparent); }
.badge.warn { color: var(--warn); background: color-mix(in srgb, var(--warn) 18%, transparent); }
.synthesis > strong { font-family: var(--font-display); text-transform: uppercase; letter-spacing: 0.05em;
  font-size: 0.8rem; display: inline-block; margin-bottom: 0.3rem; }
.synthesis ul { margin: 0.3rem 0 0; padding-left: 1.2rem; }
.synthesis.rubric ul { color: var(--ink-quiet); }
.ranking { margin: 0; }
.ranking ol { margin: 0.3rem 0 0; padding-left: 0; list-style: none; counter-reset: rank; }
.ranking ol li { counter-increment: rank; font-family: var(--font-display); font-weight: 700;
  margin: 0.35rem 0; padding-left: 2.3rem; position: relative; }
.ranking ol li::before { content: counter(rank); position: absolute; left: 0; top: -0.05rem;
  width: 1.6rem; height: 1.6rem; border-radius: var(--r-full); background: var(--ink); color: var(--bg);
  font-size: 0.82rem; display: flex; align-items: center; justify-content: center; }
.ranking ol li:first-child::before { background: var(--accent); color: var(--accent-ink); }
.reco { margin-top: 0; }
.reco ul { margin: 0.5rem 0 0.2rem; padding-left: 1.2rem; }
.reco p { margin: 0.4rem 0 0.2rem; }
/* Two-column rows to use the full width on normal screens (single column below ~800px). */
.grid-2 { display: grid; gap: 0.8rem 2rem; grid-template-columns: repeat(auto-fit, minmax(380px, 1fr));
  align-items: start; margin: 3rem 0 0; }
.grid-2 > section { min-width: 0; }
.grid-2 > section > h2:first-child { margin-top: 0; }
.grid-2.tight { margin: 1rem 0; }
.verdicts { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1rem; margin: 1rem 0; }
.verdict { background: var(--surface); border: 1px solid var(--border); border-top: 4px solid var(--ink);
  border-radius: var(--r-card); padding: 0.8rem 1.1rem; }
.verdict h3 { margin: 0.1rem 0 0.6rem; font-size: 1.05rem; display: flex; align-items: center; gap: 0.5rem; }
.verdict .score { margin-left: auto; background: var(--accent); color: var(--accent-ink); font-weight: 700;
  font-size: 0.8rem; padding: 0.12rem 0.55rem; border-radius: var(--r-full); font-variant-numeric: tabular-nums; }
.verdict-spread { margin: 0 0 0.5rem; color: var(--ink-quiet); font-family: var(--font-ui); font-size: 0.8rem;
  font-variant-numeric: tabular-nums; }
.verdict-line { margin: 0.2rem 0 0.6rem; }
.verdict ul { margin: 0.3rem 0; padding-left: 0; list-style: none; }
.verdict li { margin: 0.25rem 0; padding-left: 1.5rem; text-indent: -1.5rem; }
.verdict ul.pros li::before { content: "\2713"; color: var(--ok); font-weight: 800; margin-right: 0.6rem; }
.verdict ul.cons li::before { content: "\2717"; color: var(--err); font-weight: 800; margin-right: 0.6rem; }
.verdict ul.checks { margin: 0.6rem 0 0.2rem; padding-top: 0.6rem; border-top: 1px solid var(--border); }
.verdict ul.checks li.ok::before { content: "\2713"; color: var(--ok); font-weight: 800; margin-right: 0.6rem; }
.verdict ul.checks li.ko::before { content: "\2717"; color: var(--err); font-weight: 800; margin-right: 0.6rem; }
.verdict ul.checks li.warn::before { content: "\7e"; color: var(--warn); font-weight: 800; margin-right: 0.6rem; }
.verdict ul.checks .stat { color: var(--ink-quiet); font-size: 0.8rem; }
.params { margin: 1rem 0; }
.params dl { display: grid; grid-template-columns: max-content 1fr; gap: 0.35rem 1.2rem; margin: 0.5rem 0 0; }
.params dt { font-family: var(--font-display); font-weight: 600; font-size: 0.85rem; color: var(--ink-quiet); }
.params dd { margin: 0; min-width: 0; }
.params dd ol { margin: 0; padding-left: 1.2rem; }
.params-note { color: var(--ink-quiet); font-size: 0.82rem; margin: 0.7rem 0 0; }
table.matrix { border-collapse: collapse; width: 100%; table-layout: fixed; margin: 1rem 0;
  font-family: var(--font-ui); font-size: 0.88rem; }
table.matrix th, table.matrix td { border: 1px solid var(--border); padding: 0.5rem 0.8rem;
  text-align: left; vertical-align: top; word-break: break-word; }
table.matrix thead th { background: var(--ink); color: var(--bg); font-family: var(--font-display);
  text-transform: uppercase; letter-spacing: 0.03em; font-size: 0.76rem; }
table.matrix th:first-child, table.matrix td.crit { width: 26%; }
table.matrix td.crit { font-size: 0.86rem; color: var(--ink); }
table.matrix td.lvl .mark { font-weight: 800; margin-right: 0.4rem; }
table.matrix td.lvl .obs { color: var(--ink); }
table.matrix td.lvl.ok { background: color-mix(in srgb, var(--ok) 10%, transparent); }
table.matrix td.lvl.ok .mark { color: var(--ok); }
table.matrix td.lvl.warn { background: color-mix(in srgb, var(--accent) 14%, transparent); }
table.matrix td.lvl.warn .mark { color: var(--warn); }
table.matrix td.lvl.ko { background: color-mix(in srgb, var(--err) 10%, transparent); }
table.matrix td.lvl.ko .mark { color: var(--err); }
table.matrix td.lvl.na .mark { color: var(--ink-quiet); }
table.matrix tfoot td { font-weight: 700; background: var(--surface-quiet); }
table.matrix tfoot td.scorerow { text-align: right; font-variant-numeric: tabular-nums; }
table.matrix thead th .runs { font-family: var(--font-ui); font-weight: 400; text-transform: none;
  letter-spacing: 0; opacity: 0.7; font-size: 0.85em; }
table.matrix tfoot td .spread { color: var(--ink-quiet); font-weight: 400; font-size: 0.85em; }
.mark.ok-fg { color: var(--ok); font-weight: 800; }
.mark.ko-fg { color: var(--err); font-weight: 800; }
.card { border: 1px solid var(--border); border-radius: var(--r-card); margin: 1.2rem 0; overflow: hidden; }
.card > header { background: var(--surface-quiet); padding: 0.7rem 1rem; display: flex; align-items: center;
  gap: 0.8rem; border-bottom: 1px solid var(--border); }
.card > header h3 { margin: 0; font-size: 1.05rem; }
.card > header .spacer { flex: 1; }
.card > header .stat { color: var(--ink-quiet); font-family: var(--font-ui); font-size: 0.85rem;
  font-variant-numeric: tabular-nums; }
.card .body { padding: 0.2rem 1rem 1rem; }
.card-info { min-width: 0; }
.card-info > :first-child { margin-top: 0; }
.result p { margin: 0.6rem 0; }
.result p:first-child { margin-top: 0.4rem; }
.result ul { margin: 0.6rem 0; padding-left: 1.2rem; }
h4.detail-sub { font-family: var(--font-display); font-size: 0.9rem; margin: 1rem 0 0.4rem; }
ul.checklist { list-style: none; margin: 0.3rem 0 0; padding: 0; }
ul.checklist li { padding: 0.15rem 0; }
ul.checklist li.ok::before { content: "\2713"; color: var(--ok); font-weight: 800; margin-right: 0.6rem; }
ul.checklist li.ko::before { content: "\2717"; color: var(--err); font-weight: 800; margin-right: 0.6rem; }
ul.checklist li .stat { color: var(--ink-quiet); font-family: var(--font-mono); font-size: 0.8rem; }
p.tool-summary { font-family: var(--font-mono); font-size: 0.82rem; }
form.apply { margin: 1rem 0 0; }
.sxs-controls { display: flex; gap: 1.2rem; flex-wrap: wrap; margin: 1rem 0; align-items: center; }
.sxs-controls label { color: var(--ink-quiet); font-family: var(--font-display); font-size: 0.78rem;
  text-transform: uppercase; letter-spacing: 0.03em; display: flex; gap: 0.5rem; align-items: center; }
.sxs-controls select { font-family: var(--font-ui); font-size: 0.9rem; text-transform: none; letter-spacing: 0;
  padding: 0.35rem 0.5rem; background: var(--surface); color: var(--ink);
  border: 2px solid var(--border-strong); border-radius: var(--r-ctl); }
.sxs-file { border: 1px solid var(--border); border-radius: var(--r-card); margin: 1rem 0; overflow: hidden; }
.sxs-head { background: var(--surface-quiet); padding: 0.55rem 0.9rem; border-bottom: 1px solid var(--border);
  display: flex; align-items: center; gap: 0.7rem; }
.ftag { color: var(--ink-quiet); font-family: var(--font-display); font-weight: 600; font-size: 0.72rem;
  text-transform: uppercase; letter-spacing: 0.04em; border: 1px solid var(--border-strong);
  border-radius: var(--r-full); padding: 0.08rem 0.55rem; }
.sxs-scroll { overflow-x: auto; }
table.sxs { width: 100%; border-collapse: collapse; table-layout: fixed; margin: 0;
  font-family: var(--font-mono); font-size: 0.8rem; line-height: 1.35; }
table.sxs td { padding: 0 0.6rem; border: 0; vertical-align: top; white-space: pre-wrap; word-break: break-word; }
table.sxs td.ln { width: 3rem; text-align: right; color: var(--ink-quiet); user-select: none;
  white-space: nowrap; overflow: hidden; padding: 0 0.5rem; }
table.sxs td.code.left { border-right: 1px solid var(--border); }
/* A line present on only one side is an addition on that side (green); a line
   that differs between the two configs is highlighted amber on both sides. */
table.sxs tr.del td.left { background: var(--add-bg); color: var(--add-fg); }
table.sxs tr.add td.right { background: var(--add-bg); color: var(--add-fg); }
table.sxs tr.chg td.left, table.sxs tr.chg td.right { background: var(--chg-bg); color: var(--chg-fg); }
.ftrees { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 1rem; margin: 1rem 0; }
.ftree-card { border: 1px solid var(--border); border-radius: var(--r-ctl); padding: 0.6rem 0.9rem; min-width: 0; overflow-x: auto; }
.ftree-card h3 { margin: 0 0 0.5rem; font-size: 0.95rem; }
ul.ftree { list-style: none; margin: 0; padding-left: 0.9rem; font-family: var(--font-mono); font-size: 0.8rem; line-height: 1.5; }
.ftree li.dir { font-weight: 600; }
.ftree li.file.added { color: var(--add-fg); }
.ftree li.file.removed { color: var(--del-fg); text-decoration: line-through; }
.ftree li.file.added::before { content: "+ "; }
.ftree li.file.removed::before { content: "− "; }
.ftree li.file.modified::before { content: "~ "; color: var(--ink-quiet); }
.ftree-legend { color: var(--ink-quiet); font-size: 0.85rem; }
.ftree-legend .added { color: var(--add-fg); } .ftree-legend .removed { color: var(--del-fg); }
button.primary { font-family: var(--font-ui); font-weight: 600; font-size: 0.85rem; border: 1px solid var(--border-strong);
  background: var(--accent); color: var(--accent-ink); border-radius: var(--r-ctl); padding: 0.4rem 0.8rem; cursor: pointer; }
button.export-config { font-family: var(--font-ui); font-size: 0.85rem; border: 1px solid var(--border-strong);
  background: var(--surface); color: var(--ink); border-radius: var(--r-ctl); padding: 0.35rem 0.7rem; cursor: pointer; }
dialog.picker { border: 1px solid var(--border-strong); border-radius: var(--r-card); padding: 1rem; max-width: 560px; width: 90vw; color: var(--ink); background: var(--surface); }
dialog.picker::backdrop { background: rgba(0, 0, 0, 0.4); }
.picker-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.6rem; }
.picker-head button { border: 0; background: none; font-size: 1.3rem; line-height: 1; cursor: pointer; color: var(--ink); }
.picker-list { list-style: none; margin: 0.4rem 0; padding: 0; max-height: 50vh; overflow-y: auto; font-family: var(--font-mono); font-size: 0.82rem; }
.picker-list li { padding: 0.12rem 0; }
.picker-actions { display: flex; justify-content: flex-end; margin-top: 0.7rem; }
.cfg-all { display: block; font-family: var(--font-ui); font-size: 0.85rem; font-weight: 600; margin-bottom: 0.3rem; }
.empty { color: var(--ink-quiet); font-style: italic; }
/* Layout : contenu + sommaire latéral droit collant (masqué sous ~1080px). */
.layout { display: grid; grid-template-columns: minmax(0, 1fr); gap: 2.5rem; }
main { min-width: 0; }
main > :first-child { margin-top: 0; }
.toc { display: none; }
@media (min-width: 1080px) {
  .layout { grid-template-columns: minmax(0, 1fr) 15rem; }
  .toc { display: block; }
}
.toc-inner { position: sticky; top: 1.5rem; }
.toc-title { font-family: var(--font-display); text-transform: uppercase; letter-spacing: 0.05em;
  font-size: 0.72rem; color: var(--ink-quiet); margin: 0 0 0.7rem; }
.toc nav { display: flex; flex-direction: column; border-left: 2px solid var(--border); }
.toc nav a { color: var(--ink-quiet); text-decoration: none; font-size: 0.85rem; line-height: 1.3;
  padding: 0.3rem 0 0.3rem 0.85rem; margin-left: -2px; border-left: 2px solid transparent; }
.toc nav a:hover { color: var(--ink); }
.toc nav a.active { color: var(--ink); font-weight: 600; border-left-color: var(--accent); }
h2 { scroll-margin-top: 1.2rem; }
:focus-visible { outline: 3px solid var(--focus); outline-offset: 2px; }
@media (prefers-reduced-motion: reduce) { * { transition: none !important; animation: none !important; } }
</style>
</head>
<body>
<header class="topbar">
  <span class="brand">{{if .HomeURL}}<a href="{{.HomeURL}}">benchy</a>{{else}}benchy{{end}}</span>
  <span class="root">rapport</span>
</header>
{{if .ReuseURL}}<p class="backlink"><a href="{{.ReuseURL}}">Reprendre cette config</a></p>
{{end}}<h1>Rapport claude-benchy</h1>
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
<table>
<thead>
<tr><th>Config</th><th>Modèle</th><th>Statut</th><th>Checks</th><th>Tours</th><th>Outils</th><th>Coût</th><th>Durée</th><th>Fichiers</th><th>+Lignes</th><th>-Lignes</th>{{if .TranscriptBase}}<th>Détail</th>{{end}}</tr>
</thead>
<tbody>
{{- range .Runs}}
<tr>
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


<script>
var BENCHY_FILES = {{filesJSON .}};
(function () {
  var data = BENCHY_FILES;
  var labels = data.labels || [];
  var left = document.getElementById('sxs-left');
  var right = document.getElementById('sxs-right');
  var body = document.getElementById('sxs-body');

  if (labels.length < 2) {
    body.innerHTML = '<p class="empty">Comparaison indisponible : moins de deux configurations ont produit des fichiers.</p>';
    var controls = document.querySelector('.sxs-controls');
    if (controls) { controls.style.display = 'none'; }
    return;
  }

  labels.forEach(function (l) {
    left.add(new Option(l, l));
    right.add(new Option(l, l));
  });
  left.value = labels[0];
  right.value = labels[1];

  function esc(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function toLines(s) {
    if (!s) { return []; }
    var a = s.split('\n');
    if (a.length && a[a.length - 1] === '') { a.pop(); }
    return a;
  }

  function lcsOps(a, b) {
    var n = a.length, m = b.length, ops = [], i, j;
    if (n * m > 4000000) {
      for (i = 0; i < n; i++) { ops.push({ t: 'del', i: i }); }
      for (j = 0; j < m; j++) { ops.push({ t: 'ins', j: j }); }
      return ops;
    }
    var dp = [];
    for (i = 0; i <= n; i++) { dp[i] = new Int32Array(m + 1); }
    for (i = n - 1; i >= 0; i--) {
      for (j = m - 1; j >= 0; j--) {
        dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
      }
    }
    i = 0; j = 0;
    while (i < n && j < m) {
      if (a[i] === b[j]) { ops.push({ t: 'eq', i: i, j: j }); i++; j++; }
      else if (dp[i + 1][j] >= dp[i][j + 1]) { ops.push({ t: 'del', i: i }); i++; }
      else { ops.push({ t: 'ins', j: j }); j++; }
    }
    while (i < n) { ops.push({ t: 'del', i: i }); i++; }
    while (j < m) { ops.push({ t: 'ins', j: j }); j++; }
    return ops;
  }

  function alignRows(a, b) {
    var ops = lcsOps(a, b), rows = [], ln = 0, rn = 0, pd = [], pi = [];
    function flush() {
      var k = Math.max(pd.length, pi.length), x;
      for (x = 0; x < k; x++) {
        var L = x < pd.length ? { n: ++ln, text: a[pd[x]] } : null;
        var R = x < pi.length ? { n: ++rn, text: b[pi[x]] } : null;
        rows.push({ l: L, r: R, cls: (L && R) ? 'chg' : (L ? 'del' : 'add') });
      }
      pd = []; pi = [];
    }
    ops.forEach(function (op) {
      if (op.t === 'eq') {
        flush();
        rows.push({ l: { n: ++ln, text: a[op.i] }, r: { n: ++rn, text: b[op.j] }, cls: 'eq' });
      } else if (op.t === 'del') { pd.push(op.i); }
      else { pi.push(op.j); }
    });
    flush();
    return rows;
  }

  function cell(side, c) {
    if (!c) { return '<td class="ln"></td><td class="code ' + side + '"></td>'; }
    return '<td class="ln">' + c.n + '</td><td class="code ' + side + '">' + esc(c.text) + '</td>';
  }

  function badge(inL, inR, same) {
    var txt = !inL ? 'droite seulement' : !inR ? 'gauche seulement' : same ? 'identique' : 'divergent';
    return '<span class="ftag">' + txt + '</span>';
  }

  function render() {
    var lf = data.files[left.value] || {}, rf = data.files[right.value] || {};
    var seen = {};
    Object.keys(lf).forEach(function (k) { seen[k] = 1; });
    Object.keys(rf).forEach(function (k) { seen[k] = 1; });
    var keys = Object.keys(seen).sort();
    if (!keys.length) { body.innerHTML = '<p class="empty">Aucun fichier pour ce couple.</p>'; return; }

    var out = '';
    keys.forEach(function (path) {
      var inL = Object.prototype.hasOwnProperty.call(lf, path);
      var inR = Object.prototype.hasOwnProperty.call(rf, path);
      var lc = inL ? lf[path] : '', rc = inR ? rf[path] : '';
      out += '<div class="sxs-file">';
      out += '<div class="sxs-head"><code>' + esc(path) + '</code>' + badge(inL, inR, lc === rc) + '</div>';
      out += '<div class="sxs-scroll"><table class="sxs"><tbody>';
      alignRows(toLines(lc), toLines(rc)).forEach(function (row) {
        out += '<tr class="' + row.cls + '">' + cell('left', row.l) + cell('right', row.r) + '</tr>';
      });
      out += '</tbody></table></div></div>';
    });
    body.innerHTML = out;
  }

  left.addEventListener('change', render);
  right.addEventListener('change', render);
  render();
})();

(function () {
  var links = Array.prototype.slice.call(document.querySelectorAll('#toc-nav a'));
  if (!links.length || !('IntersectionObserver' in window)) { return; }
  var sections = links
    .map(function (a) { return document.getElementById(a.getAttribute('href').slice(1)); })
    .filter(Boolean);
  var current = null;
  function setActive(id) {
    if (id === current) { return; }
    current = id;
    links.forEach(function (a) { a.classList.toggle('active', a.getAttribute('href').slice(1) === id); });
  }
  var observer = new IntersectionObserver(function (entries) {
    entries.forEach(function (e) { if (e.isIntersecting) { setActive(e.target.id); } });
  }, { rootMargin: '0px 0px -75% 0px', threshold: 0 });
  sections.forEach(function (s) { observer.observe(s); });
})();
</script>

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
(function () {
  var dialog = document.getElementById('cfg-export');
  if (!dialog) { return; }
  var list = document.getElementById('cfg-files');
  var artifact = document.getElementById('cfg-artifact');
  var title = document.getElementById('cfg-title');
  var selectAll = document.getElementById('cfg-selectall');
  var form = document.getElementById('cfg-form');
  var dirValue = form.querySelector('[name=dir]').value;

  document.addEventListener('click', function (e) {
    var btn = e.target.closest('.export-config');
    if (!btn) { return; }
    artifact.value = btn.dataset.artifact;
    title.textContent = 'Exporter la conf — ' + btn.dataset.label;
    list.innerHTML = '<li class="muted">Chargement…</li>';
    selectAll.checked = true;
    fetch('/config-files?dir=' + encodeURIComponent(dirValue) + '&artifact=' + encodeURIComponent(btn.dataset.artifact))
      .then(function (r) { return r.ok ? r.json() : r.text().then(function (t) { throw new Error(t); }); })
      .then(function (data) {
        list.innerHTML = '';
        (data.files || []).forEach(function (f) {
          var li = document.createElement('li');
          var lab = document.createElement('label');
          var cb = document.createElement('input');
          cb.type = 'checkbox'; cb.name = 'files'; cb.value = f; cb.checked = true;
          lab.appendChild(cb); lab.appendChild(document.createTextNode(' ' + f));
          li.appendChild(lab); list.appendChild(li);
        });
      })
      .catch(function (err) { list.innerHTML = '<li class="err">' + (err.message || 'erreur') + '</li>'; });
    dialog.showModal();
  });

  selectAll.addEventListener('change', function () {
    list.querySelectorAll('input[name=files]').forEach(function (cb) { cb.checked = selectAll.checked; });
  });
  document.getElementById('cfg-close').addEventListener('click', function () { dialog.close(); });
  form.addEventListener('submit', function (e) {
    var n = list.querySelectorAll('input[name=files]:checked').length;
    if (n === 0) { e.preventDefault(); return; }
    if (!confirm('Exporter ' + n + ' fichier(s) de conf sur le projet testé ? Les fichiers existants seront écrasés (non committé).')) {
      e.preventDefault();
    }
  });
})();
</script>
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
	return fm
}
