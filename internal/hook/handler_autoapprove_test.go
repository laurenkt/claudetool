package hook

import (
	"bytes"
	"strings"
	"testing"
)

// runAutoApprove runs the handler and reports whether it emitted an "allow"
// permission decision on stdout.
func runAutoApprove(t *testing.T, command string) bool {
	t.Helper()
	input := makeToolInput("PreToolUse", "Bash", BashInput{Command: command})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"auto-approve-readonly"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	return strings.Contains(stdout.String(), `"permissionDecision":"allow"`)
}

func TestAutoApproveAllows(t *testing.T) {
	cmds := []string{
		// The infuriating real case from the screenshot.
		`cd /Users/x/worktrees/analytics-foo && execute-in-data-shell adbt dataset 2>&1 | tail -1`,
		`cd /repo && echo "===" && find dbt/models -iname "*open_banking*" | grep -iE "\.sql$" | head`,
		`grep -rl "connected_on_date" dbt/models --include="*.sql" 2>/dev/null | head`,
		"ls -la && pwd",
		"cat file.sql | head -50",
		"git status && git log --oneline -10",
		"git diff HEAD~1 -- path/to/file.sql",
		"gh pr view 123 && gh pr checks",
		"gh api repos/monzo/analytics/pulls/1",
		`FOO=bar echo hi`,
		"find . -name '*.yml'",
		"sort file.txt | uniq -c | sort -rn",
		// bq metadata reads (no scan cost) — the screenshot case.
		`cd /repo && bq show --format=prettyjson monzo-analytics:episodes.foo 2>/dev/null | jq -r '.schema.fields[].name'`,
		"bq ls monzo-analytics:episodes",
		"bq --format=prettyjson show foo",
		"bq head -n 5 monzo-analytics:episodes.foo",
		"bq version",
		"diff a.sql b.sql",
		// Quote-aware splitting: pipes INSIDE a jq program must not split.
		`cd /repo && bq show --format=prettyjson x 2>/dev/null | jq -r '[.schema.fields[] | select(.name|test("a|b|c"))|.name] | join(", ")'`,
		`echo "a | b ; c && d"`,
		`grep 'foo|bar' file.sql`,
		// Extra read-only utilities from the sweep.
		"md5 -q /tmp/x.md",
		"cd /repo && ps aux | grep dbt",
		// Richer git read coverage.
		"git branch | grep pdg",
		"git branch -a",
		"git stash list",
		"git worktree list",
		"git config --get remote.origin.url",
		"git merge-base main HEAD",
		// Data-shell read verbs.
		"execute-in-data-shell adbt upstream some_model",
		"execute-in-data-shell run-data-standards static",
		"execute-in-data-shell model-routing get-routing foo",
		// Bare data tools (not wrapped in execute-in-data-shell) — screenshot case.
		"model-routing get-routing --model-group monzo-analytics-v2 --model-type concept 2>&1 | head -20",
		"adbt upstream some_model",
		"run-data-standards static",
		"modelgen version",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			if !runAutoApprove(t, c) {
				t.Errorf("command should be auto-approved: %q", c)
			}
		})
	}
}

func TestAutoApproveStaysSilent(t *testing.T) {
	cmds := []string{
		// Mutating verbs.
		"cd /repo && rm -rf build/",
		"git push origin HEAD",
		"gh pr merge 123",
		"mv a b",
		// Write forms of otherwise-read commands.
		"echo hi > realfile.txt",
		"sort -o out.txt in.txt",
		"find . -name '*.tmp' -delete",
		`find . -name '*.log' -exec rm {} \;`,
		// gh api with a mutating body.
		"gh api -X POST repos/monzo/analytics/issues",
		"gh api repos/x -f title=hi",
		// Data-shell query execution (cost/mutation risk) is not auto-approved.
		"execute-in-data-shell adbt run --select foo",
		"execute-in-data-shell bq query 'select 1'",
		// bq query scans data (costs money) — must still prompt.
		"bq query --use_legacy_sql=false 'select 1'",
		"bq --format=prettyjson query 'select count(*) from t'",
		// bq write/DDL subcommands must still prompt.
		"bq rm -t monzo-analytics:episodes.foo",
		"bq mk --table monzo-analytics:episodes.bar",
		"bq load ds.table gs://x.csv",
		// git write forms must still prompt even with the richer git handler.
		"git branch newfeature",
		"git branch -D oldbranch",
		"git stash pop",
		"git worktree add ../wt main",
		"git config user.email x@y.com",
		// A real command substitution inside double quotes must NOT be approved.
		`echo "$(rm -rf /tmp/x)"`,
		// base64 writing to a file, and data-shell arbitrary bash.
		"base64 -o out.bin in",
		"execute-in-data-shell bash 'rm -rf x'",
		"execute-in-data-shell adbt run --select foo",
		// Substitutions hide nested commands.
		"echo $(rm -rf /)",
		"cat <(rm x)",
		"echo `whoami`",
		// Unknown command.
		"terraform apply",
		"./deploy.sh",
		// git write subcommand.
		"git commit -m x",
		"git branch -D main",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			if runAutoApprove(t, c) {
				t.Errorf("command should NOT be auto-approved: %q", c)
			}
		})
	}
}

func TestAutoApproveIgnoresNonBashAndNonPre(t *testing.T) {
	// Non-Bash tool: silent.
	if got := runAutoApprove(t, ""); got {
		t.Error("empty command should not be approved")
	}
	// PostToolUse event should be ignored even for a read-only command.
	input := makeToolInput("PostToolUse", "Bash", BashInput{Command: "ls"})
	var stdout, stderr bytes.Buffer
	Run([]string{"auto-approve-readonly"}, strings.NewReader(input), &stdout, &stderr)
	if strings.Contains(stdout.String(), "allow") {
		t.Error("PostToolUse should not be auto-approved")
	}
}
