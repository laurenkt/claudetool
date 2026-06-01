package hook

import (
	"strings"
	"testing"
)

func TestUseLinearMCPBlocksBash(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"gh issue view linear", "gh issue view --repo foo/linear-issues 123"},
		{"gh api linear", `gh api repos/foo/linear-issues/issues/1`},
		{"curl linear.app", "curl -sSL https://linear.app/foo/issue/ABC-123"},
		{"wget linear.app", "wget https://linear.app/foo/issue/ABC-123"},
		{"linear-cli issue list", "linear-cli issue list"},
		{"linear-cli short alias", "linear-cli i v ABC-123"},
		{"linear-cli piped", "linear-cli issue list | head"},
		{"linear-cli after &&", `git status && linear-cli issue list`},
		{"linear-cli relative path", "./linear-cli issue list"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: tt.command})
			code, stderr := runHandler(t, "use-linear-mcp", input)
			if code != 2 {
				t.Errorf("exit code = %d, want 2 (block); stderr=%q", code, stderr)
			}
			if !strings.Contains(stderr, "Linear MCP") {
				t.Errorf("stderr = %q, want Linear MCP message", stderr)
			}
		})
	}
}

func TestUseLinearMCPBlocksWebFetch(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"linear.app issue", "https://linear.app/foo/issue/ABC-123"},
		{"workspace subdomain", "https://monzo.linear.app/team/ABC/issue/ABC-123"},
		{"mixed case", "https://LINEAR.app/foo/issue/ABC-123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "WebFetch", WebFetchInput{URL: tt.url, Prompt: "summarise"})
			code, stderr := runHandler(t, "use-linear-mcp", input)
			if code != 2 {
				t.Errorf("exit code = %d, want 2 (block); stderr=%q", code, stderr)
			}
			if !strings.Contains(stderr, "Linear MCP") {
				t.Errorf("stderr = %q, want Linear MCP message", stderr)
			}
		})
	}
}

func TestUseLinearMCPAllows(t *testing.T) {
	for _, cmd := range []string{
		"git status",
		"gh pr list",
		"curl https://example.com",
	} {
		t.Run("allow bash: "+cmd, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: cmd})
			if code, stderr := runHandler(t, "use-linear-mcp", input); code != 0 {
				t.Errorf("exit code = %d, want 0; stderr=%q", code, stderr)
			}
		})
	}

	t.Run("bash mentioning linear without trigger", func(t *testing.T) {
		input := makeToolInput("PreToolUse", "Bash", BashInput{Command: `echo "linear algebra"`})
		if code, stderr := runHandler(t, "use-linear-mcp", input); code != 0 {
			t.Errorf("exit code = %d, want 0; stderr=%q", code, stderr)
		}
	})

	t.Run("linear-cli substring in unrelated tool", func(t *testing.T) {
		input := makeToolInput("PreToolUse", "Bash", BashInput{Command: "my-linear-cli-helper foo"})
		if code, stderr := runHandler(t, "use-linear-mcp", input); code != 0 {
			t.Errorf("exit code = %d, want 0 (substring should not match); stderr=%q", code, stderr)
		}
	})

	t.Run("non-linear webfetch", func(t *testing.T) {
		input := makeToolInput("PreToolUse", "WebFetch", WebFetchInput{URL: "https://example.com/foo", Prompt: "summarise"})
		if code, stderr := runHandler(t, "use-linear-mcp", input); code != 0 {
			t.Errorf("exit code = %d, want 0; stderr=%q", code, stderr)
		}
	})

	t.Run("write tool ignored", func(t *testing.T) {
		input := makeToolInput("PreToolUse", "Write", WriteInput{FilePath: "/tmp/x.txt", Content: "linear-cli stuff https://linear.app"})
		if code, stderr := runHandler(t, "use-linear-mcp", input); code != 0 {
			t.Errorf("exit code = %d, want 0 (Write should be ignored); stderr=%q", code, stderr)
		}
	})

	t.Run("edit tool ignored", func(t *testing.T) {
		input := makeToolInput("PreToolUse", "Edit", EditInput{FilePath: "/tmp/x.txt", OldString: "a", NewString: "linear-cli"})
		if code, stderr := runHandler(t, "use-linear-mcp", input); code != 0 {
			t.Errorf("exit code = %d, want 0 (Edit should be ignored); stderr=%q", code, stderr)
		}
	})
}
