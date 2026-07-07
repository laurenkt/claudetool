package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// runStripCd runs the handler with a given cwd and returns the rewritten
// command (from updatedInput), or "" if the handler produced no rewrite.
func runStripCd(t *testing.T, command, cwd string) string {
	t.Helper()
	ti, _ := json.Marshal(BashInput{Command: command})
	in := Input{HookEventName: "PreToolUse", ToolName: "Bash", ToolInput: ti, CWD: cwd}
	data, _ := json.Marshal(in)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"strip-redundant-cd"}, strings.NewReader(string(data)), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr=%q", code, stderr.String())
	}
	if stdout.Len() == 0 {
		return ""
	}
	var out Output
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("bad output json: %v (%s)", err, stdout.String())
	}
	if out.HookSpecificOutput == nil || out.HookSpecificOutput.UpdatedInput == nil {
		return ""
	}
	cmd, _ := out.HookSpecificOutput.UpdatedInput["command"].(string)
	return cmd
}

func TestStripRedundantCd(t *testing.T) {
	cwd := "/Users/j/src/github.com/monzo/worktrees/analytics/pdg-3764"
	cases := []struct {
		name, command, cwd, want string
	}{
		{
			name:    "screenshot: cd cwd + git checkout compound",
			command: `cd ` + cwd + ` && git checkout master -- "a.sql" "a.yml" && echo "=== x ===" && git status --short`,
			cwd:     cwd,
			want:    `git checkout master -- "a.sql" "a.yml" && echo "=== x ===" && git status --short`,
		},
		{
			name:    "newline separator",
			command: "cd " + cwd + "\ngit status",
			cwd:     cwd,
			want:    "git status",
		},
		{
			name:    "trailing slash still matches",
			command: "cd " + cwd + "/ && ls",
			cwd:     cwd,
			want:    "ls",
		},
		{
			name:    "quoted dir",
			command: `cd "` + cwd + `" && ls`,
			cwd:     cwd,
			want:    "ls",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runStripCd(t, tc.command, tc.cwd); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStripRedundantCdLeavesAlone(t *testing.T) {
	cwd := "/Users/j/repo"
	cases := []struct{ name, command, cwd string }{
		{"different dir - real cd", "cd /Users/j/other && ls", cwd},
		{"relative cd", "cd subdir && ls", cwd},
		{"bare cd no command", "cd /Users/j/repo", cwd},
		{"cd - ", "cd - && ls", cwd},
		{"no cd at all", "git status && ls", cwd},
		{"empty cwd", "cd /Users/j/repo && ls", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runStripCd(t, tc.command, tc.cwd); got != "" {
				t.Errorf("expected no rewrite, got %q", got)
			}
		})
	}
}
