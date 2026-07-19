package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keyxmare/claude-benchy/internal/claude"
	"github.com/keyxmare/claude-benchy/internal/diffcap"
)

func TestResultHTMLCollapsesSoftWraps(t *testing.T) {
	got := string(resultHTML("Ligne un\nligne deux.\n\n**Gras** et `code`."))
	want := "<p>Ligne un ligne deux.</p><p><strong>Gras</strong> et <code>code</code>.</p>"
	if got != want {
		t.Errorf("resultHTML() =\n%s\nwant\n%s", got, want)
	}
}

func TestParseChanges(t *testing.T) {
	added := "diff --git a/docs/x.md b/docs/x.md\nnew file mode 100644\n--- /dev/null\n+++ b/docs/x.md\n@@ -0,0 +1 @@\n+hi\n"
	removed := "diff --git a/old.go b/old.go\ndeleted file mode 100644\n--- a/old.go\n+++ /dev/null\n"
	modified := "diff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md\n@@ -1 +1 @@\n-a\n+b\n"

	malformed := "diff --git bogus-line-without-b-path\n"
	got := parseChanges(added + removed + modified + malformed)

	want := []fileChange{
		{Path: "README.md", Status: "modified"},
		{Path: "docs/x.md", Status: "added"},
		{Path: "old.go", Status: "removed"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseChanges() mismatch (-want +got):\n%s", diff)
	}
	if n := parseChanges(""); n != nil {
		t.Errorf("parseChanges(empty) = %+v, want nil", n)
	}
}

func TestChangeSym(t *testing.T) {
	for status, want := range map[string]string{"added": "+", "removed": "-", "modified": "~", "": "~"} {
		if got := changeSym(status); got != want {
			t.Errorf("changeSym(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestAggregateLevel(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"empty", nil, ""},
		{"unanimous met", []string{"respecté", "respecté"}, "respecté"},
		{"met boundary 0.75", []string{"respecté", "respecté", "respecté", "non"}, "respecté"},
		{"mixed to partial", []string{"respecté", "non"}, "partiel"},
		{"unanimous unmet", []string{"non", "non"}, "non"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := aggregateLevel(tt.in); got != tt.want {
				t.Errorf("aggregateLevel(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestInlineHTMLLeavesUnclosedSpansClosed(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"unclosed code", "`code", "<code>code</code>"},
		{"unclosed bold", "**bold", "<strong>bold</strong>"},
		{"closed pair", "a `b` **c**", "a <code>b</code> <strong>c</strong>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := inlineHTML(tt.in); got != tt.want {
				t.Errorf("inlineHTML(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestResultHTMLBulletList(t *testing.T) {
	got := string(resultHTML("- premier\n* second `x`"))
	want := "<ul><li>premier</li><li>second <code>x</code></li></ul>"
	if got != want {
		t.Errorf("resultHTML() = %q, want %q", got, want)
	}
}

func TestIsBulletList(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want bool
	}{
		{"all bullets", []string{"- a", "* b"}, true},
		{"one non-bullet", []string{"- a", "prose"}, false},
		{"empty", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isBulletList(tt.in); got != tt.want {
				t.Errorf("isBulletList(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		name string
		run  RunReport
		want string
	}{
		{"hard error", RunReport{Err: "boom"}, "erreur"},
		{"claude error", RunReport{Metrics: claude.Metrics{IsError: true}}, "erreur-claude"},
		{"degenerate", RunReport{Metrics: claude.Metrics{ToolUses: 0}, Diff: diffcap.Stats{FilesChanged: 0}}, "sans effet"},
		{"ok", RunReport{Metrics: claude.Metrics{ToolUses: 3}, Diff: diffcap.Stats{FilesChanged: 1}}, "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := status(tt.run); got != tt.want {
				t.Errorf("status(%+v) = %q, want %q", tt.run, got, tt.want)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	if got := add(2, 3); got != 5 {
		t.Errorf("add(2, 3) = %d, want 5", got)
	}
}

func TestRelDelta(t *testing.T) {
	tests := []struct {
		name    string
		base, v float64
		want    float64
	}{
		{"zero base short-circuits", 0, 5, 0},
		{"increase", 10, 15, 50},
		{"decrease", 10, 5, -50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := relDelta(tt.base, tt.v); got != tt.want {
				t.Errorf("relDelta(%v, %v) = %v, want %v", tt.base, tt.v, got, tt.want)
			}
		})
	}
}

func TestOverBest(t *testing.T) {
	tests := []struct {
		name        string
		best, worst float64
		want        string
	}{
		{"zero best is n/a", 0, 5, "n/a"},
		{"overshoot", 10, 15, "+50 % vs le meilleur"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := overBest(tt.best, tt.worst); got != tt.want {
				t.Errorf("overBest(%v, %v) = %q, want %q", tt.best, tt.worst, got, tt.want)
			}
		})
	}
}

func TestSignedPct(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want string
	}{
		{"positive", 42, "+42 %"},
		{"zero", 0, "+0 %"},
		{"negative", -12, "-12 %"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := signedPct(tt.in); got != tt.want {
				t.Errorf("signedPct(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSlowerFaster(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want string
	}{
		{"slower", 30, "30 % plus lentes"},
		{"flat", 0, "0 % plus lentes"},
		{"faster", -25, "25 % plus rapides"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := slowerFaster(tt.in); got != tt.want {
				t.Errorf("slowerFaster(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestChangeDelta(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want string
	}{
		{"wider", 12, "un périmètre plus large (+12 ligne(s) en moyenne)"},
		{"narrower", -8, "un périmètre plus étroit (-8 ligne(s) en moyenne)"},
		{"comparable", 0, "un périmètre de changement comparable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := changeDelta(tt.in); got != tt.want {
				t.Errorf("changeDelta(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFirstLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"multiline keeps first", "premier\nsecond", "premier"},
		{"single line unchanged", "seul", "seul"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := firstLine(tt.in); got != tt.want {
				t.Errorf("firstLine(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"empty", nil, ""},
		{"single", []string{"a"}, "a"},
		{"multiple", []string{"a", "b", "c"}, "a, b, c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := join(tt.in); got != tt.want {
				t.Errorf("join(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestFilesJSON exercises the two data-driven branches: a run with files is
// serialised, a run without files is skipped. The json.Marshal error return is
// unreachable — the payload is a map[string]map[string]string of Go strings,
// which json.Marshal never fails to encode — so it stays justified, not tested.
func TestFilesJSON(t *testing.T) {
	r := Report{Runs: []RunReport{
		{Config: "vanilla", Run: 1, Files: []FileVersion{{Path: "a.go", Content: "x"}}},
		{Config: "empty", Run: 1},
	}}

	var got struct {
		Labels []string                     `json:"labels"`
		Files  map[string]map[string]string `json:"files"`
	}
	if err := json.Unmarshal([]byte(filesJSON(r)), &got); err != nil {
		t.Fatalf("filesJSON produced invalid JSON: %v", err)
	}

	want := struct {
		Labels []string
		Files  map[string]map[string]string
	}{
		Labels: []string{"vanilla"},
		Files:  map[string]map[string]string{"vanilla": {"a.go": "x"}},
	}
	if diff := cmp.Diff(want.Labels, got.Labels); diff != "" {
		t.Errorf("filesJSON labels mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(want.Files, got.Files); diff != "" {
		t.Errorf("filesJSON files mismatch (-want +got):\n%s", diff)
	}
}

func TestFileTreeHTML(t *testing.T) {
	patch := "diff --git a/docs/a/b.md b/docs/a/b.md\nnew file mode 100644\n" +
		"diff --git a/old.go b/old.go\ndeleted file mode 100644\n" +
		"diff --git a/README.md b/README.md\n@@ -1 +1 @@\n"

	got := string(fileTreeHTML(RunReport{Patch: patch}))

	for _, want := range []string{
		`<li class="dir">docs/`,
		`<li class="dir">a/`,
		`<li class="file added">b.md</li>`,
		`<li class="file removed">old.go</li>`,
		`<li class="file modified">README.md</li>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("fileTreeHTML() missing %q in:\n%s", want, got)
		}
	}
	if empty := string(fileTreeHTML(RunReport{})); !strings.Contains(empty, "Aucune modification") {
		t.Errorf("fileTreeHTML(empty) = %q", empty)
	}
}
