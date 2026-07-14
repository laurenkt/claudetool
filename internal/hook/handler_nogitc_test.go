package hook

import (
	"strings"
	"testing"
)

// A `git -C <path>` invocation is blocked wherever it sits at a command
// position, since git run in the working directory already targets the right
// repository and `-C` only adds an approval-tripping directory change.
func TestNoGitCBlocks(t *testing.T) {
	cmds := []string{
		"git -C /repo status",
		"git -C ../other log --oneline",
		"ls && git -C . diff",
		"git   -C /path commit -m x",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, stderr := runHandler(t, "no-git-c", input)
			if code != 2 {
				t.Errorf("exit = %d, want 2 (block)", code)
			}
			if !strings.Contains(stderr, "git -C") {
				t.Errorf("stderr = %q, want git -C guidance", stderr)
			}
		})
	}
}

// Ordinary git commands, git's lowercase `-c` config option, and a `-C`
// belonging to a subcommand (git blame's copy detection) are all left alone.
func TestNoGitCAllows(t *testing.T) {
	cmds := []string{
		"git status",
		"git log --oneline",
		"git -c core.pager=cat log",
		"git blame -C file.go",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, _ := runHandler(t, "no-git-c", input)
			if code != 0 {
				t.Errorf("exit = %d, want 0 (allow)", code)
			}
		})
	}
}
