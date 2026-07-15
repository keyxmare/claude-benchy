package report

import (
	"fmt"
	htmltmpl "html/template"
	texttmpl "text/template"
)

func status(r RunReport) string {
	if r.Err != "" {
		return "error"
	}
	if r.Metrics.IsError {
		return "claude-error"
	}
	return "ok"
}

func cost(r RunReport) string {
	return fmt.Sprintf("$%.4f", r.Metrics.TotalCostUSD)
}

func seconds(ms int) string {
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

var funcs = map[string]any{
	"status":  status,
	"cost":    cost,
	"seconds": seconds,
}

const markdownSource = `# claude-benchy report

- **Generated:** {{.GeneratedAt}}
- **App:** {{.App}}

## Prompt

` + "```" + `
{{.Prompt}}
` + "```" + `

## Comparison

| Config | Run | Model | Status | Turns | Tools | Cost | Duration | Files | +Lines | -Lines |
|--------|-----|-------|--------|-------|-------|------|----------|-------|--------|--------|
{{- range .Runs}}
| {{.Config}} | {{.Run}} | {{.Model}} | {{status .}} | {{.Metrics.NumTurns}} | {{.Metrics.ToolUses}} | {{cost .}} | {{seconds .Metrics.DurationMS}} | {{.Diff.FilesChanged}} | {{.Diff.Insertions}} | {{.Diff.Deletions}} |
{{- end}}

## Runs
{{range .Runs}}
### {{.Config}} (run {{.Run}})

- **Artifacts:** {{.ArtifactDir}}
{{- if .Err}}
- **Error:** {{.Err}}
{{- end}}
{{- if .Metrics.Result}}
- **Result:** {{.Metrics.Result}}
{{- end}}
{{end}}
`

const htmlSource = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>claude-benchy report</title>
<style>
body { font-family: system-ui, sans-serif; margin: 2rem; color: #1a1a1a; }
table { border-collapse: collapse; width: 100%; margin: 1rem 0; }
th, td { border: 1px solid #ddd; padding: 0.4rem 0.6rem; text-align: left; }
th { background: #f4f4f4; }
pre { background: #f6f8fa; padding: 1rem; border-radius: 6px; overflow-x: auto; }
.ok { color: #1a7f37; }
.error, .claude-error { color: #cf222e; }
code { font-family: ui-monospace, monospace; }
</style>
</head>
<body>
<h1>claude-benchy report</h1>
<p><strong>Generated:</strong> {{.GeneratedAt}}<br>
<strong>App:</strong> <code>{{.App}}</code></p>
<h2>Prompt</h2>
<pre>{{.Prompt}}</pre>
<h2>Comparison</h2>
<table>
<thead>
<tr><th>Config</th><th>Run</th><th>Model</th><th>Status</th><th>Turns</th><th>Tools</th><th>Cost</th><th>Duration</th><th>Files</th><th>+Lines</th><th>-Lines</th></tr>
</thead>
<tbody>
{{- range .Runs}}
<tr>
<td>{{.Config}}</td><td>{{.Run}}</td><td>{{.Model}}</td>
<td class="{{status .}}">{{status .}}</td>
<td>{{.Metrics.NumTurns}}</td><td>{{.Metrics.ToolUses}}</td>
<td>{{cost .}}</td><td>{{seconds .Metrics.DurationMS}}</td>
<td>{{.Diff.FilesChanged}}</td><td>{{.Diff.Insertions}}</td><td>{{.Diff.Deletions}}</td>
</tr>
{{- end}}
</tbody>
</table>
<h2>Runs</h2>
{{range .Runs}}
<h3>{{.Config}} (run {{.Run}})</h3>
<ul>
<li><strong>Artifacts:</strong> <code>{{.ArtifactDir}}</code></li>
{{- if .Err}}
<li class="error"><strong>Error:</strong> {{.Err}}</li>
{{- end}}
{{- if .Metrics.Result}}
<li><strong>Result:</strong> {{.Metrics.Result}}</li>
{{- end}}
</ul>
{{end}}
</body>
</html>
`

var (
	markdownTemplate = texttmpl.Must(texttmpl.New("md").Funcs(funcs).Parse(markdownSource))
	htmlTemplate     = htmltmpl.Must(htmltmpl.New("html").Funcs(htmltmpl.FuncMap(funcs)).Parse(htmlSource))
)
