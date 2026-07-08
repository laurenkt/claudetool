package hook

import (
	"strings"
	"testing"
)

func TestNoCommandSubstitutionBlocks(t *testing.T) {
	cmds := []string{
		// The screenshot case.
		`docker inspect --format '{{.Name}}' $(docker ps -q --filter "label=is-dbt-monzo-shell=true") | grep pdg-3722`,
		`bq query "$(cat /tmp/q.sql)"`,
		"git checkout $(git merge-base main HEAD)",
		"echo `whoami`",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, stderr := runHandler(t, "no-command-substitution", input)
			if code != 2 {
				t.Errorf("exit = %d, want 2 (block)", code)
			}
			if !strings.Contains(stderr, "command substitution") {
				t.Errorf("stderr = %q, want command-substitution guidance", stderr)
			}
		})
	}
}

func TestNoCommandSubstitutionAllows(t *testing.T) {
	cmds := []string{
		// The two-step rewrite.
		"docker ps -q --filter label=is-dbt-monzo-shell=true",
		"docker inspect --format '{{.Name}} {{.State.Status}}' abc123 def456",
		// Arithmetic is allowed.
		"echo $((1 + 2))",
		// Plain variable expansion is not command substitution.
		"git -C \"$WT\" status",
		"grep foo file.sql",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, _ := runHandler(t, "no-command-substitution", input)
			if code != 0 {
				t.Errorf("exit = %d, want 0 (allow)", code)
			}
		})
	}
}
