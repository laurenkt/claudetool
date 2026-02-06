package statusline

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInputDecode(t *testing.T) {
	raw := `{
		"hook_event_name": "Status",
		"session_id": "abc-123",
		"model": {"id": "claude-opus-4-6", "display_name": "Opus"},
		"workspace": {"current_dir": "/tmp/proj", "project_dir": "/tmp/proj"},
		"cost": {"total_cost_usd": 1.23, "total_lines_added": 10},
		"context_window": {
			"used_percentage": 42.5,
			"remaining_percentage": 57.5,
			"current_usage": {
				"input_tokens": 1000,
				"output_tokens": 500
			}
		}
	}`

	var in Input
	if err := json.NewDecoder(strings.NewReader(raw)).Decode(&in); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if in.Model.DisplayName != "Opus" {
		t.Errorf("model display_name = %q, want Opus", in.Model.DisplayName)
	}
	if in.Cost.TotalCostUSD != 1.23 {
		t.Errorf("cost = %v, want 1.23", in.Cost.TotalCostUSD)
	}
	if in.ContextWindow.UsedPercentage != 42.5 {
		t.Errorf("used_percentage = %v, want 42.5", in.ContextWindow.UsedPercentage)
	}
	if in.ContextWindow.CurrentUsage == nil {
		t.Fatal("current_usage is nil")
	}
	if in.ContextWindow.CurrentUsage.InputTokens != 1000 {
		t.Errorf("input_tokens = %d, want 1000", in.ContextWindow.CurrentUsage.InputTokens)
	}
}

func TestInputDecodeNullCurrentUsage(t *testing.T) {
	raw := `{
		"model": {"display_name": "Opus"},
		"context_window": {"used_percentage": 10, "current_usage": null}
	}`
	var in Input
	if err := json.NewDecoder(strings.NewReader(raw)).Decode(&in); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if in.ContextWindow.CurrentUsage != nil {
		t.Error("expected current_usage to be nil")
	}
}
