// Package claude parses the stream-json transcript emitted by `claude -p` and
// extracts the final result metrics.
package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// Usage holds token accounting reported by Claude.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// Metrics summarises a headless Claude run.
type Metrics struct {
	IsError       bool    `json:"is_error"`
	Subtype       string  `json:"subtype"`
	DurationMS    int     `json:"duration_ms"`
	DurationAPIMS int     `json:"duration_api_ms"`
	NumTurns      int     `json:"num_turns"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
	Result        string  `json:"result"`
	Usage         Usage   `json:"usage"`
	ToolUses      int     `json:"tool_uses"`
	// ToolBreakdown counts tool_use blocks per tool name (e.g. Bash, Edit). It
	// is derived while parsing, not part of the result event, and persisted in
	// result.json so the report can detail which tools a run leaned on.
	ToolBreakdown map[string]int `json:"tool_breakdown,omitempty"`
}

const maxLineBytes = 16 * 1024 * 1024

type envelope struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"content"`
	} `json:"message"`
}

// Parse reads a stream-json transcript and returns the metrics from the final
// result event, together with the number of tool_use blocks seen along the way.
func Parse(r io.Reader) (Metrics, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	var (
		metrics   Metrics
		toolUses  int
		breakdown = map[string]int{}
		found     bool
	)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var env envelope
		if err := json.Unmarshal(line, &env); err != nil {
			continue
		}
		switch env.Type {
		case "assistant":
			for _, block := range env.Message.Content {
				if block.Type == "tool_use" {
					toolUses++
					breakdown[block.Name]++
				}
			}
		case "result":
			if err := json.Unmarshal(line, &metrics); err != nil {
				return Metrics{}, fmt.Errorf("parse result event: %w", err)
			}
			found = true
		}
	}
	if err := scanner.Err(); err != nil {
		return Metrics{}, err
	}
	if !found {
		return Metrics{}, fmt.Errorf("no result event found in transcript")
	}
	metrics.ToolUses = toolUses
	if len(breakdown) > 0 {
		metrics.ToolBreakdown = breakdown
	}
	return metrics, nil
}
