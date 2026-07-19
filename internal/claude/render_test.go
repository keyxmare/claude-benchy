package claude_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keyxmare/claude-benchy/internal/claude"
)

func TestRenderAssistantTextAndTools(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{
		{
			"init dropped",
			`{"type":"system","subtype":"init","model":"claude-sonnet"}`,
			nil,
		},
		{
			"text",
			`{"type":"assistant","message":{"content":[{"type":"text","text":"  J'ajoute les tests.  "}]}}`,
			[]string{"J'ajoute les tests."},
		},
		{
			"bash",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}`,
			[]string{"⏺ Bash(go test ./...)"},
		},
		{
			"edit with snippet",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"util/scale.go","old_string":"return v*f","new_string":"return v*f + off"}}]}}`,
			[]string{"⏺ Edit(util/scale.go)", "  ⎿ return v*f → return v*f + off"},
		},
		{
			"edit empty strings yields no snippet",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"a.go","old_string":"","new_string":""}}]}}`,
			[]string{"⏺ Edit(a.go)"},
		},
		{
			"edit multiline old keeps first line",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"a.go","old_string":"foo\nbar","new_string":"baz"}}]}}`,
			[]string{"⏺ Edit(a.go)", "  ⎿ foo → baz"},
		},
		{
			"bash non-string command",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":123}}]}}`,
			[]string{"⏺ Bash()"},
		},
		{
			"read",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"main.go"}}]}}`,
			[]string{"⏺ Read(main.go)"},
		},
		{
			"write",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"out.go"}}]}}`,
			[]string{"⏺ Write(out.go)"},
		},
		{
			"grep",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Grep","input":{"pattern":"func"}}]}}`,
			[]string{"⏺ Grep(func)"},
		},
		{
			"glob",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Glob","input":{"pattern":"*.go"}}]}}`,
			[]string{"⏺ Glob(*.go)"},
		},
		{
			"unknown tool with input",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Task","input":{"prompt":"do it"}}]}}`,
			[]string{`⏺ Task({"prompt":"do it"})`},
		},
		{
			"unknown tool no input",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Task","input":{}}]}}`,
			[]string{"⏺ Task"},
		},
		{
			"unknown tool absent input field",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Task"}]}}`,
			[]string{"⏺ Task"},
		},
		{
			"tool result",
			`{"type":"user","message":{"content":[{"type":"tool_result","content":"ok util\nFAIL geometry"}]}}`,
			[]string{"  ⎿ ok util …(+1 lignes)"},
		},
		{
			"tool result error",
			`{"type":"user","message":{"content":[{"type":"tool_result","is_error":true,"content":[{"type":"text","text":"boom"}]}]}}`,
			[]string{"  ⎿ ⚠ boom"},
		},
		{
			"user non-tool_result block dropped",
			`{"type":"user","message":{"content":[{"type":"text","text":"hi"}]}}`,
			nil,
		},
		{
			"tool result absent content dropped",
			`{"type":"user","message":{"content":[{"type":"tool_result"}]}}`,
			nil,
		},
		{
			"tool result non-textual content dropped",
			`{"type":"user","message":{"content":[{"type":"tool_result","content":42}]}}`,
			nil,
		},
		{
			"result success",
			`{"type":"result","subtype":"success","num_turns":3,"total_cost_usd":1.2393}`,
			[]string{"✓ terminé · 3 tours · $1.2393"},
		},
		{
			"result error",
			`{"type":"result","subtype":"error_max_turns","is_error":true}`,
			[]string{"✗ échec (error_max_turns)"},
		},
		{
			"result error without subtype",
			`{"type":"result","is_error":true}`,
			[]string{"✗ échec"},
		},
		{"unknown type", `{"type":"whatever"}`, nil},
		{"blank", "   ", nil},
		{"garbage", "not json", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := claude.Render([]byte(tc.line))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Render() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLiveWriterSplitsAndFlushes(t *testing.T) {
	var got []string
	w := claude.NewLiveWriter(func(s string) { got = append(got, s) })

	// A tool event split across two Writes, then a partial trailing line.
	_, _ = w.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bas`))
	_, _ = w.Write([]byte("h\",\"input\":{\"command\":\"ls\"}}]}}\n{\"type\":\"result\",\"subtype\":\"success\",\"num_turns\":1,\"total_cost_usd\":0.5}"))

	if want := []string{"⏺ Bash(ls)"}; cmp.Diff(want, got) != "" {
		t.Errorf("before flush =\n%q\nwant %q", got, want)
	}
	w.Flush()
	if want := []string{"⏺ Bash(ls)", "✓ terminé · 1 tours · $0.5000"}; cmp.Diff(want, got) != "" {
		t.Errorf("after flush =\n%q\nwant %q", got, want)
	}
}
