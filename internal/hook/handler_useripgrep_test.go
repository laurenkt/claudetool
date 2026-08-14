package hook

import (
	"strings"
	"testing"
)

// A grep/egrep/fgrep invocation is blocked wherever it sits at a command
// position, since ripgrep is a faster, .gitignore-aware replacement.
func TestUseRipgrepBlocks(t *testing.T) {
	cmds := []string{
		"grep foo file.txt",
		"grep -r foo .",
		"egrep 'foo|bar' file.txt",
		"fgrep foo file.txt",
		"ls && grep foo file.txt",
		"cat file.txt | grep foo",
		"echo hi; grep foo file.txt",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, stderr := runHandler(t, "use-ripgrep", input)
			if code != 2 {
				t.Errorf("exit = %d, want 2 (block)", code)
			}
			if !strings.Contains(stderr, "ripgrep") {
				t.Errorf("stderr = %q, want ripgrep guidance", stderr)
			}
		})
	}
}

// Tools whose names merely end in "grep" (ripgrep itself, zgrep, pgrep) and
// mentions of the word "grep" inside a quoted argument are left alone.
func TestUseRipgrepAllows(t *testing.T) {
	cmds := []string{
		"rg foo file.txt",
		"zgrep foo file.txt.gz",
		"pgrep -f myproc",
		"echo 'grep is a tool'",
		"echo mygrep",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, _ := runHandler(t, "use-ripgrep", input)
			if code != 0 {
				t.Errorf("exit = %d, want 0 (allow)", code)
			}
		})
	}
}

func TestUseRipgrepIgnoresNonBash(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/tmp/test.txt",
		Content:  "grep foo bar",
	})
	code, _ := runHandler(t, "use-ripgrep", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (ignore non-Bash)", code)
	}
}
