package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// renderEvent decodes the fields of a stream-json event needed to render a
// readable feed line — a superset of the metrics envelope.
type renderEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Model   string `json:"model"`
	Message struct {
		Content []renderBlock `json:"content"`
	} `json:"message"`
	NumTurns     int     `json:"num_turns"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	IsError      bool    `json:"is_error"`
}

type renderBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Content json.RawMessage `json:"content"`
	IsError bool            `json:"is_error"`
}

// Render turns one stream-json transcript line into zero or more human-readable
// feed lines: assistant text, tool calls with their inputs, tool results and
// the final outcome. Unrecognised or empty events yield nothing.
func Render(line []byte) []string {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return nil
	}
	var e renderEvent
	if err := json.Unmarshal(line, &e); err != nil {
		return nil
	}
	switch e.Type {
	case "system":
		// The init banner (model, tools, cwd…) is orchestration noise already
		// shown in the run's top console; the agent feed shows only its work.
		return nil
	case "assistant":
		return renderAssistant(e.Message.Content)
	case "user":
		return renderResults(e.Message.Content)
	case "result":
		return []string{renderResult(e)}
	}
	return nil
}

func renderAssistant(blocks []renderBlock) []string {
	var out []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if t := strings.TrimSpace(b.Text); t != "" {
				out = append(out, t)
			}
		case "tool_use":
			out = append(out, renderToolUse(b.Name, b.Input)...)
		}
	}
	return out
}

func renderToolUse(name string, input json.RawMessage) []string {
	m := map[string]any{}
	_ = json.Unmarshal(input, &m)
	switch name {
	case "Bash":
		return []string{"⏺ Bash(" + truncate(str(m["command"]), 140) + ")"}
	case "Edit", "MultiEdit":
		lines := []string{"⏺ " + name + "(" + str(m["file_path"]) + ")"}
		if d := editSnippet(m); d != "" {
			lines = append(lines, "  ⎿ "+d)
		}
		return lines
	case "Write":
		return []string{"⏺ Write(" + str(m["file_path"]) + ")"}
	case "Read":
		return []string{"⏺ Read(" + str(m["file_path"]) + ")"}
	case "Grep":
		return []string{"⏺ Grep(" + truncate(str(m["pattern"]), 100) + ")"}
	case "Glob":
		return []string{"⏺ Glob(" + truncate(str(m["pattern"]), 100) + ")"}
	default:
		if s := compact(input); s != "" && s != "{}" {
			return []string{"⏺ " + name + "(" + truncate(s, 120) + ")"}
		}
		return []string{"⏺ " + name}
	}
}

// editSnippet summarises an Edit as the first changed line, old → new.
func editSnippet(m map[string]any) string {
	oldLine, newLine := firstLine(str(m["old_string"])), firstLine(str(m["new_string"]))
	if oldLine == "" && newLine == "" {
		return ""
	}
	return truncate(oldLine, 60) + " → " + truncate(newLine, 60)
}

func renderResults(blocks []renderBlock) []string {
	var out []string
	for _, b := range blocks {
		if b.Type != "tool_result" {
			continue
		}
		text := strings.TrimSpace(extractResultText(b.Content))
		if text == "" {
			continue
		}
		prefix := "  ⎿ "
		if b.IsError {
			prefix = "  ⎿ ⚠ "
		}
		out = append(out, prefix+summarise(text, 160))
	}
	return out
}

func renderResult(e renderEvent) string {
	if e.IsError || (e.Subtype != "" && e.Subtype != "success") {
		if e.Subtype != "" {
			return "✗ échec (" + e.Subtype + ")"
		}
		return "✗ échec"
	}
	return fmt.Sprintf("✓ terminé · %d tours · $%.4f", e.NumTurns, e.TotalCostUSD)
}

// extractResultText pulls the text out of a tool_result content, which is
// either a plain string or an array of typed blocks.
func extractResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var b strings.Builder
		for _, blk := range blocks {
			if blk.Type == "text" {
				b.WriteString(blk.Text)
			}
		}
		return b.String()
	}
	return ""
}

// summarise flattens text to its first non-empty line, capped, noting how many
// further lines were elided.
func summarise(s string, max int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	first := ""
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			first = strings.TrimSpace(l)
			break
		}
	}
	out := truncate(first, max)
	if n := len(lines) - 1; n > 0 {
		out += fmt.Sprintf(" …(+%d lignes)", n)
	}
	return out
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}

func compact(raw json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return ""
	}
	return b.String()
}

// LiveWriter renders a stream-json byte stream into readable feed lines on the
// fly: it buffers partial writes, splits on newlines and emits each event's
// rendered lines through emit. Call Flush to render a trailing partial line.
type LiveWriter struct {
	buf  []byte
	emit func(string)
}

// NewLiveWriter returns a LiveWriter forwarding rendered lines to emit.
func NewLiveWriter(emit func(string)) *LiveWriter {
	return &LiveWriter{emit: emit}
}

func (w *LiveWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.emitLine(w.buf[:i])
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

// Flush renders any buffered bytes that were not newline-terminated.
func (w *LiveWriter) Flush() {
	if len(bytes.TrimSpace(w.buf)) > 0 {
		w.emitLine(w.buf)
	}
	w.buf = nil
}

func (w *LiveWriter) emitLine(line []byte) {
	for _, out := range Render(line) {
		w.emit(out)
	}
}
