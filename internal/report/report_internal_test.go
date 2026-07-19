package report

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
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
