package hook

import (
	"strings"
	"testing"
)

func TestNoProdRegistered(t *testing.T) {
	if _, ok := registry["no-prod"]; !ok {
		t.Fatal("no-prod not registered")
	}
}

func TestNoProdBlocks(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"short prod", `£ -e prod "restart service.foo"`},
		{"short production", `£ -e production "restart service.foo"`},
		{"long prod", `£ --environment prod "restart service.foo"`},
		{"long production", `£ --environment production "restart service.foo"`},
		{"short equals", `£ -e=prod "restart service.foo"`},
		{"long equals", `£ --environment=production "restart service.foo"`},
		{"tabs", "£\t-e\tprod\t\"restart service.foo\""},
		{"in chain", `git status && £ -e prod "deploy"`},
		{"after semicolon", `echo hi; £ -e prod "deploy"`},
		{"after pipe", `bla bla | £ -e prod "deploy"`},
		{"leading whitespace", `    £ -e prod "deploy"`},
		{"command substitution", `result=$(£ -e prod "deploy")`},
		{"backticks", "echo `£ -e prod \"deploy\"`"},
		{"sudo wrapper", `sudo £ -e prod "deploy"`},
		{"xargs wrapper", `xargs £ -e prod`},
		{"time wrapper", `time £ -e prod "deploy"`},
		{"env-var prefix", `FOO=bar £ -e prod "deploy"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: tt.command})
			code, stderr := runHandler(t, "no-prod", input)
			if code != 2 {
				t.Errorf("exit code = %d, want 2 (block); stderr=%q", code, stderr)
			}
			if !strings.Contains(stderr, "STRICTLY forbidden") {
				t.Errorf("stderr = %q, want the forbidden message", stderr)
			}
			if !strings.Contains(stderr, "AskUserQuestion") {
				t.Errorf("stderr = %q, want mention of AskUserQuestion", stderr)
			}
		})
	}
}

func TestNoProdAllows(t *testing.T) {
	for _, cmd := range []string{
		`£ -e s101 "restart service.foo"`,    // lower environment
		`£ -e staging "restart service.foo"`, // lower environment
		`£ --environment s101 "deploy"`,      // lower environment
		`grep -e prod config.yaml`,           // grep -e is an expression, not the £ CLI
		`sed -e prod-thing file`,             // sed -e is a script, not the £ CLI
		`echo "£ -e prod is forbidden"`,      // documentation string, not a £ invocation
		`£ -e prodigy "deploy"`,              // prodigy is not prod/production
		`git status`,
	} {
		t.Run("allow: "+cmd, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: cmd})
			if code, stderr := runHandler(t, "no-prod", input); code != 0 {
				t.Errorf("exit code = %d, want 0; stderr=%q", code, stderr)
			}
		})
	}
}

func TestNoProdIgnoresNonBash(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/tmp/x.md",
		Content:  `£ -e prod "deploy"`,
	})
	if code, stderr := runHandler(t, "no-prod", input); code != 0 {
		t.Errorf("exit code = %d, want 0 (non-Bash ignored); stderr=%q", code, stderr)
	}
}
