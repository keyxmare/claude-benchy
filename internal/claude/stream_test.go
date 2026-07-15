package claude

import (
	"os"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	f, err := os.Open("testdata/transcript.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	m, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if m.IsError {
		t.Error("expected IsError false")
	}
	if m.NumTurns != 3 {
		t.Errorf("NumTurns = %d, want 3", m.NumTurns)
	}
	if m.ToolUses != 2 {
		t.Errorf("ToolUses = %d, want 2", m.ToolUses)
	}
	if m.TotalCostUSD != 0.0123 {
		t.Errorf("TotalCostUSD = %v, want 0.0123", m.TotalCostUSD)
	}
	if m.Usage.InputTokens != 1500 || m.Usage.OutputTokens != 420 {
		t.Errorf("unexpected usage: %+v", m.Usage)
	}
	if m.Result != "Done." {
		t.Errorf("Result = %q", m.Result)
	}
}

func TestParseNoResult(t *testing.T) {
	_, err := Parse(strings.NewReader(`{"type":"system"}` + "\n"))
	if err == nil {
		t.Fatal("expected error when no result event is present")
	}
}

func TestParseIgnoresGarbageLines(t *testing.T) {
	in := "not json\n" + `{"type":"result","num_turns":1,"is_error":true}` + "\n"
	m, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsError || m.NumTurns != 1 {
		t.Errorf("unexpected metrics: %+v", m)
	}
}
