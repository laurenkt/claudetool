package hook

import (
	"strings"
	"testing"
)

func TestNoShellLoopsBlocks(t *testing.T) {
	cmds := []string{
		// The real screenshot pattern.
		`cd /repo && for t in prod.a prod.b; do echo "=== $t ==="; bq show monzo-analytics:$t; done`,
		"for f in *.sql; do grep x $f; done",
		"while read line; do echo $line; done < in.txt",
		"until false; do :; done",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, stderr := runHandler(t, "no-shell-loops", input)
			if code != 2 {
				t.Errorf("exit = %d, want 2 (block)", code)
			}
			if !strings.Contains(stderr, "unanalyzable") {
				t.Errorf("stderr = %q, want reformulation guidance", stderr)
			}
		})
	}
}

func TestNoShellLoopsAllows(t *testing.T) {
	cmds := []string{
		"ls -la && pwd",
		"grep for somefile.sql",          // 'for' is an argument, not a loop
		"bq show monzo-analytics:prod.a", // single call, no loop
		"echo before && echo after",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, _ := runHandler(t, "no-shell-loops", input)
			if code != 0 {
				t.Errorf("exit = %d, want 0 (allow)", code)
			}
		})
	}
}

func TestNoInlinePythonBlocks(t *testing.T) {
	cmds := []string{
		`bq show --format=prettyjson monzo-analytics:prod.a | python3 -c "import json,sys; print(json.load(sys.stdin))"`,
		`python -c 'print(1)'`,
		`python2 -c "print 1"`,
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, stderr := runHandler(t, "no-inline-python", input)
			if code != 2 {
				t.Errorf("exit = %d, want 2 (block)", code)
			}
			if !strings.Contains(stderr, "jq") {
				t.Errorf("stderr = %q, want jq suggestion", stderr)
			}
		})
	}
}

func TestNoInlinePythonAllows(t *testing.T) {
	cmds := []string{
		"python3 script.py",        // running a file, not inline
		"python3 -m pytest",        // module, not -c
		"bq show x | jq '.schema'", // the recommended form
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, _ := runHandler(t, "no-inline-python", input)
			if code != 0 {
				t.Errorf("exit = %d, want 0 (allow)", code)
			}
		})
	}
}

func TestNoInlineFileBlocks(t *testing.T) {
	cmds := []string{
		// The real screenshot patterns.
		"cat > /tmp/cov.sql <<'SQL'\nSELECT 1\nSQL",
		`bq query --use_legacy_sql=false --format=prettyjson "$(cat /tmp/cov.sql)"`,
		"bq query \"`cat /tmp/cov.sql`\"",
		"cat <<EOF\nhi\nEOF",
		// Commit-message / PR-body string-builders via command substitution.
		`git add -A && git commit -q -m "$(printf 'title\n\nbody line')"`,
		"gh pr create --body \"`printf 'a\\nb'`\"",
		`echo "$(echo hi)"`,
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, stderr := runHandler(t, "no-inline-file", input)
			if code != 2 {
				t.Errorf("exit = %d, want 2 (block)", code)
			}
			if !strings.Contains(stderr, "Write tool") {
				t.Errorf("stderr = %q, want Write-tool guidance", stderr)
			}
		})
	}
}

func TestNoExitCodeCheckBlocks(t *testing.T) {
	cmds := []string{
		`echo "EXIT=$?"`,
		`bq query --use_legacy_sql=false < /tmp/q.sql > /tmp/out.json 2>/tmp/err.txt; echo "EXIT=$?"; cat /tmp/out.json`,
		"make build; echo $?",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, stderr := runHandler(t, "no-exit-code-check", input)
			if code != 2 {
				t.Errorf("exit = %d, want 2 (block)", code)
			}
			if !strings.Contains(stderr, "exit-code scaffolding") {
				t.Errorf("stderr = %q, want exit-code guidance", stderr)
			}
		})
	}
}

func TestNoExitCodeCheckAllows(t *testing.T) {
	cmds := []string{
		"bq query --use_legacy_sql=false --format=prettyjson < /tmp/q.sql",
		"git status && echo done",
		"grep foo file.sql",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, _ := runHandler(t, "no-exit-code-check", input)
			if code != 0 {
				t.Errorf("exit = %d, want 0 (allow)", code)
			}
		})
	}
}

func TestNoInlineFileAllows(t *testing.T) {
	cmds := []string{
		// The recommended rewrites.
		"bq query --use_legacy_sql=false --format=prettyjson < /tmp/cov.sql",
		"git commit -F /tmp/msg.txt",
		`git commit -m "title" -m "body paragraph"`,
		"gh pr create --body-file /tmp/body.md",
		"cat /tmp/cov.sql",      // plain read, no inlining
		"grep foo /tmp/cov.sql", // plain read
		"echo done",             // plain echo, not a $(echo ...) builder
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, _ := runHandler(t, "no-inline-file", input)
			if code != 0 {
				t.Errorf("exit = %d, want 0 (allow)", code)
			}
		})
	}
}
