package report

import (
	"fmt"
	"html"
	htmltmpl "html/template"
	"strings"
	texttmpl "text/template"
)

func status(r RunReport) string {
	if r.Err != "" {
		return "erreur"
	}
	if r.Metrics.IsError {
		return "erreur-claude"
	}
	return "ok"
}

func statusClass(r RunReport) string {
	if r.OK() {
		return "ok"
	}
	return "err"
}

func cost(r RunReport) string {
	return fmt.Sprintf("$%.4f", r.Metrics.TotalCostUSD)
}

func seconds(ms int) string {
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

// diffHTML renders a unified diff as a colourised, HTML-escaped block.
func diffHTML(patch string) htmltmpl.HTML {
	if strings.TrimSpace(patch) == "" {
		return htmltmpl.HTML(`<p class="empty">Aucune modification.</p>`)
	}
	var b strings.Builder
	b.WriteString(`<pre class="diff">`)
	for _, line := range strings.Split(patch, "\n") {
		class := "ctx"
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"),
			strings.HasPrefix(line, "diff "), strings.HasPrefix(line, "index "):
			class = "meta"
		case strings.HasPrefix(line, "@@"):
			class = "hunk"
		case strings.HasPrefix(line, "+"):
			class = "add"
		case strings.HasPrefix(line, "-"):
			class = "del"
		}
		fmt.Fprintf(&b, `<span class="%s">%s</span>`+"\n", class, html.EscapeString(line))
	}
	b.WriteString("</pre>")
	return htmltmpl.HTML(b.String())
}

// resultHTML renders Claude's result summary with minimal inline formatting.
func resultHTML(text string) htmltmpl.HTML {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	escaped := html.EscapeString(text)
	var b strings.Builder
	code := false
	for _, r := range escaped {
		if r == '`' {
			if code {
				b.WriteString("</code>")
			} else {
				b.WriteString("<code>")
			}
			code = !code
			continue
		}
		if r == '\n' {
			b.WriteString("<br>")
			continue
		}
		b.WriteRune(r)
	}
	if code {
		b.WriteString("</code>")
	}
	return htmltmpl.HTML(b.String())
}

var funcs = map[string]any{
	"status":      status,
	"statusClass": statusClass,
	"cost":        cost,
	"seconds":     seconds,
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

| Config | Run | Modèle | Statut | Tours | Outils | Coût | Durée | Fichiers | +Lignes | -Lignes |
|--------|-----|--------|--------|-------|--------|------|-------|----------|---------|---------|
{{- range .Runs}}
| {{.Config}} | {{.Run}} | {{.Model}} | {{status .}} | {{.Metrics.NumTurns}} | {{.Metrics.ToolUses}} | {{cost .}} | {{seconds .Metrics.DurationMS}} | {{.Diff.FilesChanged}} | {{.Diff.Insertions}} | {{.Diff.Deletions}} |
{{- end}}

## Détail par config
{{range .Runs}}
### {{.Config}} (run {{.Run}}) — {{status .}}

- **Artefacts :** {{.ArtifactDir}}
{{- if .Err}}
- **Erreur :** {{.Err}}
{{- end}}
{{- if .Metrics.Result}}
- **Résultat :** {{.Metrics.Result}}
{{- end}}

` + "```diff" + `
{{.Patch}}
` + "```" + `
{{end}}
`

const htmlSource = `<!doctype html>
<html lang="fr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Rapport claude-benchy</title>
<style>
:root {
  --bg: #ffffff; --fg: #1f2328; --muted: #656d76; --border: #d0d7de;
  --card: #f6f8fa; --accent: #0969da;
  --add-bg: #e6ffec; --add-fg: #116329; --del-bg: #ffebe9; --del-fg: #82071e;
  --hunk: #0550ae; --ok: #1a7f37; --err: #cf222e;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #0d1117; --fg: #e6edf3; --muted: #8b949e; --border: #30363d;
    --card: #161b22; --accent: #4493f8;
    --add-bg: #12261e; --add-fg: #3fb950; --del-bg: #25171c; --del-fg: #f85149;
    --hunk: #58a6ff; --ok: #3fb950; --err: #f85149;
  }
}
* { box-sizing: border-box; }
body { font-family: system-ui, -apple-system, sans-serif; margin: 0; padding: 2rem;
  background: var(--bg); color: var(--fg); line-height: 1.5; max-width: 1100px; margin: 0 auto; }
h1 { margin-top: 0; }
h2 { border-bottom: 1px solid var(--border); padding-bottom: 0.3rem; margin-top: 2.5rem; }
a { color: var(--accent); }
.meta-list { color: var(--muted); font-size: 0.9rem; }
.prompt { background: var(--card); border: 1px solid var(--border); border-radius: 8px;
  padding: 0.8rem 1rem; white-space: pre-wrap; }
.synthesis { background: var(--card); border: 1px solid var(--border); border-left: 4px solid var(--accent);
  border-radius: 8px; padding: 0.6rem 1.4rem; }
table { border-collapse: collapse; width: 100%; margin: 1rem 0; font-size: 0.92rem; }
th, td { border: 1px solid var(--border); padding: 0.45rem 0.7rem; text-align: left; }
th { background: var(--card); }
td.num { text-align: right; font-variant-numeric: tabular-nums; }
.badge { font-weight: 600; padding: 0.1rem 0.5rem; border-radius: 999px; font-size: 0.82rem; }
.badge.ok { color: var(--ok); background: color-mix(in srgb, var(--ok) 15%, transparent); }
.badge.err { color: var(--err); background: color-mix(in srgb, var(--err) 15%, transparent); }
.card { border: 1px solid var(--border); border-radius: 10px; margin: 1.2rem 0; overflow: hidden; }
.card > header { background: var(--card); padding: 0.7rem 1rem; display: flex; align-items: center;
  gap: 0.8rem; border-bottom: 1px solid var(--border); }
.card > header h3 { margin: 0; font-size: 1.05rem; }
.card > header .spacer { flex: 1; }
.card > header .stat { color: var(--muted); font-size: 0.86rem; font-variant-numeric: tabular-nums; }
.card .body { padding: 0 1rem; }
.result { color: var(--fg); }
code { font-family: ui-monospace, SFMono-Regular, monospace; background: var(--card);
  padding: 0.1rem 0.3rem; border-radius: 4px; font-size: 0.9em; }
pre.diff { background: var(--card); border: 1px solid var(--border); border-radius: 8px;
  padding: 0.8rem 1rem; overflow-x: auto; font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 0.82rem; line-height: 1.45; }
pre.diff span { display: block; white-space: pre; }
pre.diff .add { background: var(--add-bg); color: var(--add-fg); }
pre.diff .del { background: var(--del-bg); color: var(--del-fg); }
pre.diff .hunk { color: var(--hunk); }
pre.diff .meta { color: var(--muted); }
.empty { color: var(--muted); font-style: italic; }
</style>
</head>
<body>
<h1>Rapport claude-benchy</h1>
<p class="meta-list"><strong>Généré :</strong> {{.GeneratedAt}} &nbsp;·&nbsp; <strong>App :</strong> <code>{{.App}}</code></p>

<h2>Prompt</h2>
<div class="prompt">{{.Prompt}}</div>

<h2>Synthèse</h2>
<ul class="synthesis">
{{- range .Synthesis}}
<li>{{. | markdownInline}}</li>
{{- end}}
</ul>

<h2>Comparaison</h2>
<table>
<thead>
<tr><th>Config</th><th>Modèle</th><th>Statut</th><th>Tours</th><th>Outils</th><th>Coût</th><th>Durée</th><th>Fichiers</th><th>+Lignes</th><th>-Lignes</th></tr>
</thead>
<tbody>
{{- range .Runs}}
<tr>
<td>{{.Config}}{{if gt .Run 0}} <span class="stat">#{{.Run}}</span>{{end}}</td>
<td>{{.Model}}</td>
<td><span class="badge {{statusClass .}}">{{status .}}</span></td>
<td class="num">{{.Metrics.NumTurns}}</td>
<td class="num">{{.Metrics.ToolUses}}</td>
<td class="num">{{cost .}}</td>
<td class="num">{{seconds .Metrics.DurationMS}}</td>
<td class="num">{{.Diff.FilesChanged}}</td>
<td class="num">{{.Diff.Insertions}}</td>
<td class="num">{{.Diff.Deletions}}</td>
</tr>
{{- end}}
</tbody>
</table>

<h2>Détail par config</h2>
{{range .Runs}}
<div class="card">
<header>
<h3>{{.Config}}{{if gt .Run 0}} · run {{.Run}}{{end}}</h3>
<span class="badge {{statusClass .}}">{{status .}}</span>
<span class="spacer"></span>
<span class="stat">{{seconds .Metrics.DurationMS}} · {{cost .}} · {{.Metrics.NumTurns}} tours · +{{.Diff.Insertions}}/-{{.Diff.Deletions}}</span>
</header>
<div class="body">
<p class="meta-list">Artefacts : <code>{{.ArtifactDir}}</code></p>
{{- if .Err}}
<p class="badge err">Erreur : {{.Err}}</p>
{{- end}}
{{- if .Metrics.Result}}
<p class="result">{{.Metrics.Result | resultHTML}}</p>
{{- end}}
{{ diffHTML .Patch }}
</div>
</div>
{{end}}
</body>
</html>
`

// markdownInline turns the small subset of Markdown used in synthesis bullets
// (**bold**) into safe HTML.
func markdownInline(s string) htmltmpl.HTML {
	escaped := html.EscapeString(s)
	var b strings.Builder
	bold := false
	for i := 0; i < len(escaped); i++ {
		if i+1 < len(escaped) && escaped[i] == '*' && escaped[i+1] == '*' {
			if bold {
				b.WriteString("</strong>")
			} else {
				b.WriteString("<strong>")
			}
			bold = !bold
			i++
			continue
		}
		b.WriteByte(escaped[i])
	}
	if bold {
		b.WriteString("</strong>")
	}
	return htmltmpl.HTML(b.String())
}

var (
	markdownTemplate = texttmpl.Must(texttmpl.New("md").Funcs(funcs).Parse(markdownSource))
	htmlTemplate     = htmltmpl.Must(htmltmpl.New("html").Funcs(htmlFuncs()).Parse(htmlSource))
)

func htmlFuncs() htmltmpl.FuncMap {
	fm := htmltmpl.FuncMap{}
	for k, v := range funcs {
		fm[k] = v
	}
	fm["diffHTML"] = diffHTML
	fm["resultHTML"] = resultHTML
	fm["markdownInline"] = markdownInline
	return fm
}
