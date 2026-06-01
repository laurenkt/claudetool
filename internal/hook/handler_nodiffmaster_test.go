package hook

import (
	"strings"
	"testing"
)

func TestNoDiffMasterBlocks(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"master...HEAD", "git diff master...HEAD"},
		{"main...HEAD", "git diff main...HEAD"},
		{"origin/master...HEAD", "git diff origin/master...HEAD"},
		{"origin/main...HEAD", "git diff origin/main...HEAD"},
		{"with --stat flag", "git diff --stat master...HEAD"},
		{"compound &&", `git diff master...HEAD --stat && echo "---" && git diff master...HEAD`},
		{"piped", "git diff master...HEAD | head -100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: tt.command})
			code, stderr := runHandler(t, "no-diff-master", input)
			if code != 2 {
				t.Errorf("exit code = %d, want 2 (block)", code)
			}
			if !strings.Contains(stderr, "not useful in a large monorepo") {
				t.Errorf("stderr = %q, want monorepo message", stderr)
			}
		})
	}
}

func TestNoDiffMasterAllows(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"diff HEAD~1", "git diff HEAD~1"},
		{"diff specific file", "git diff somefile.go"},
		{"git log master...HEAD", "git log master...HEAD"},
		{"two-dot diff with path", "git diff master -- path/to/file"},
		{"diff between commits", "git diff abc123 def456"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: tt.command})
			code, _ := runHandler(t, "no-diff-master", input)
			if code != 0 {
				t.Errorf("exit code = %d, want 0 (allow)", code)
			}
		})
	}
}

func TestNoDiffMasterIgnoresNonBash(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", map[string]string{
		"file_path": "/tmp/test.txt",
		"content":   "git diff master...HEAD",
	})
	code, _ := runHandler(t, "no-diff-master", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (ignore non-Bash)", code)
	}
}
