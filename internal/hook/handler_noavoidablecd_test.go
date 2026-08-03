package hook

import (
	"strings"
	"testing"
)

func TestNoAvoidableCDBlocks(t *testing.T) {
	cmds := []string{
		// The screenshot case: cd into the repo purely to fetch + view a PR.
		`cd /Users/x/src/github.com/monzo/analytics && git fetch --quiet origin && echo "===== PR #77251 =====" && gh pr view 77251 --json number,title,state 2>/dev/null`,
		"cd /repo && git status",
		"cd /repo && git fetch origin && git log --oneline -5",
		"cd /repo && gh pr checks",
		`cd /repo && echo "==" && git rev-parse HEAD`,
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, stderr := runHandler(t, "no-avoidable-cd", input)
			if code != 2 {
				t.Errorf("exit = %d, want 2 (block)", code)
			}
			if !strings.Contains(stderr, "git -C") {
				t.Errorf("stderr = %q, want git -C guidance", stderr)
			}
		})
	}
}

func TestNoAvoidableCDAllows(t *testing.T) {
	cmds := []string{
		// The desired rewrite — no cd, targets the repo directly.
		"git -C /repo fetch origin",
		`git -C /repo status && gh pr view 77251 -R monzo/analytics`,
		// cd is genuinely needed: a script / build / data tool / relative path.
		"cd /repo && ./generate.sh",
		"cd /repo && dbt run --select foo",
		"cd /repo && adbt upstream some_model",
		"cd /repo && make build",
		"cd /repo && find . -name '*.sql' | head",
		// A pipe consumer that could read a relative file — stay silent, don't block.
		"cd /repo && git log --oneline | grep fix",
		// Bare cd with no git/gh — nothing to nudge.
		"cd /repo",
		"cd /repo && ls -la",
		// No cd at all.
		"grep -r foo dbt/models",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, _ := runHandler(t, "no-avoidable-cd", input)
			if code != 0 {
				t.Errorf("exit = %d, want 0 (allow/silent)", code)
			}
		})
	}
}
