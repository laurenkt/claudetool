package hook

import (
	"strings"
	"testing"
)

func TestNoSharedPRBodyRegistered(t *testing.T) {
	if _, ok := registry["no-shared-pr-body"]; !ok {
		t.Fatal("no-shared-pr-body not registered")
	}
}

func TestNoSharedPRBodyBlocks(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"gh body-file space", `gh --repo monzo/wearedev pr create --draft --title "x" --body-file /tmp/pr-body.md`},
		{"gh body-file equals", `gh pr edit 123 --body-file=/tmp/pr-body.md`},
		{"gh body-file quoted", `gh pr create --body-file "/tmp/pr-body.md"`},
		{"write redirect", `cat <<'EOF' > /tmp/pr-body.md` + "\nbody\nEOF"},
		{"piped tail", `gh pr create --body-file /tmp/pr-body.md 2>&1 | tail -5`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: tt.command})
			code, stderr := runHandler(t, "no-shared-pr-body", input)
			if code != 2 {
				t.Errorf("exit code = %d, want 2 (block); stderr=%q", code, stderr)
			}
			if !strings.Contains(stderr, "/tmp/pr-body.md") {
				t.Errorf("stderr = %q, want mention of the path", stderr)
			}
		})
	}
}

func TestNoSharedPRBodyAllows(t *testing.T) {
	for _, cmd := range []string{
		`gh pr create --body-file "$(mktemp)"`,
		`body=$(mktemp); gh pr create --body-file "$body"`,
		`gh pr create --body-file /tmp/pr-body-$$.md`, // unique per-process path
		`gh pr create --body-file /tmp/pr-body.markdown`,
		`gh pr list`,
		`echo "/tmp/pr-body.txt"`,
	} {
		t.Run("allow: "+cmd, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: cmd})
			if code, stderr := runHandler(t, "no-shared-pr-body", input); code != 0 {
				t.Errorf("exit code = %d, want 0; stderr=%q", code, stderr)
			}
		})
	}
}

func TestNoSharedPRBodyIgnoresNonBash(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/tmp/pr-body.md",
		Content:  "a PR body",
	})
	if code, stderr := runHandler(t, "no-shared-pr-body", input); code != 0 {
		t.Errorf("exit code = %d, want 0 (non-Bash ignored); stderr=%q", code, stderr)
	}
}
