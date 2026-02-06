package statusline

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSub []string // substrings expected in output
		wantNot []string // substrings NOT expected in output
	}{
		{
			name: "full output",
			input: `{
				"model": {"display_name": "Opus"},
				"workspace": {"current_dir": "/tmp"},
				"cost": {"total_cost_usd": 0.5},
				"context_window": {"used_percentage": 30}
			}`,
			wantSub: []string{"/tmp", "Opus", "$0.50", "30%"},
		},
		{
			name: "cost shown when tiny",
			input: `{
				"model": {"display_name": "Sonnet"},
				"workspace": {"current_dir": "/tmp"},
				"cost": {"total_cost_usd": 0.001},
				"context_window": {"used_percentage": 10}
			}`,
			wantSub: []string{"Sonnet", "10%", "$0.00"},
		},
		{
			name: "high context yellow",
			input: `{
				"model": {"display_name": "Opus"},
				"workspace": {"current_dir": "/tmp"},
				"cost": {"total_cost_usd": 0},
				"context_window": {"used_percentage": 35}
			}`,
			wantSub: []string{"35%", "\033[33m"}, // yellow
		},
		{
			name: "critical context red+bold",
			input: `{
				"model": {"display_name": "Opus"},
				"workspace": {"current_dir": "/tmp"},
				"cost": {"total_cost_usd": 0},
				"context_window": {"used_percentage": 55}
			}`,
			wantSub: []string{"55%", "\033[1m\033[31m"}, // bold+red
		},
		{
			name: "fallback to cwd",
			input: `{
				"cwd": "/var/folders",
				"model": {"display_name": "Haiku"},
				"cost": {"total_cost_usd": 0},
				"context_window": {"used_percentage": 0}
			}`,
			wantSub: []string{"/var/folders", "Haiku"},
		},
		{
			name: "duration in minutes",
			input: `{
				"model": {"display_name": "Opus"},
				"workspace": {"current_dir": "/tmp"},
				"cost": {"total_cost_usd": 0, "total_duration_ms": 300000},
				"context_window": {"used_percentage": 10}
			}`,
			wantSub: []string{"5m"},
		},
		{
			name: "duration in hours and minutes",
			input: `{
				"model": {"display_name": "Opus"},
				"workspace": {"current_dir": "/tmp"},
				"cost": {"total_cost_usd": 0, "total_duration_ms": 5580000},
				"context_window": {"used_percentage": 10}
			}`,
			wantSub: []string{"1h33m"},
		},
		{
			name: "duration zero not shown",
			input: `{
				"model": {"display_name": "Opus"},
				"workspace": {"current_dir": "/tmp"},
				"cost": {"total_cost_usd": 0, "total_duration_ms": 0},
				"context_window": {"used_percentage": 10}
			}`,
			wantNot: []string{"\033[90m0m"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := Run(strings.NewReader(tt.input), &buf)
			if err != nil {
				t.Fatalf("Run() error: %v", err)
			}
			out := buf.String()
			for _, s := range tt.wantSub {
				if !strings.Contains(out, s) {
					t.Errorf("output %q missing substring %q", out, s)
				}
			}
			for _, s := range tt.wantNot {
				if strings.Contains(out, s) {
					t.Errorf("output %q should not contain %q", out, s)
				}
			}
		})
	}
}

func TestRunInvalidJSON(t *testing.T) {
	var buf bytes.Buffer
	err := Run(strings.NewReader("not json"), &buf)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
