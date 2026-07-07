package hook

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func init() {
	Register("no-shell-loops", handleNoShellLoops)
	Register("no-inline-python", handleNoInlinePython)
	Register("no-inline-file", handleNoInlineFile)
	Register("no-exit-code-check", handleNoExitCodeCheck)
}

// handleNoExitCodeCheck blocks `$?` exit-code scaffolding (e.g.
// `echo "EXIT=$?"`). The `$?` expansion is unanalyzable (trips the
// "simple_expansion" gate) and the check is unnecessary: the harness already
// surfaces a command's output and exit status. Run the command directly and
// read its result instead.
//
// Use as a PreToolUse hook with matcher "Bash".
func handleNoExitCodeCheck(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	if strings.Contains(bash.Command, "$?") {
		return nil, fmt.Errorf(
			"blocked: `$?` exit-code scaffolding (e.g. `echo \"EXIT=$?\"`) is unnecessary " +
				"and trips the \"simple_expansion\" approval gate. Just run the command " +
				"directly — the harness shows you its output and whether it failed. If you " +
				"sent output to a file, read it back with the Read tool, not `cat`/`tail`. " +
				"E.g. `bq query --use_legacy_sql=false --format=prettyjson < /tmp/q.sql`, then read the result.")
	}
	return nil, nil
}

// heredocRe matches a heredoc/here-string opener (<<DELIM, <<'DELIM', <<-EOF).
var heredocRe = regexp.MustCompile(`<<-?\s*['"]?\w`)

// contentSubstRe matches building command input or arguments via command
// substitution of a content emitter — $(cat ...), $(printf ...), $(echo ...)
// or their backtick forms. These inline content (file bodies, commit
// messages, PR bodies) into the command and trip Claude Code's "cannot be
// statically analyzed" gate.
var contentSubstRe = regexp.MustCompile("(\\$\\(|`)\\s*(cat|printf|echo)\\b")

// handleNoInlineFile blocks inlining content into a command via a heredoc or
// $(cat/printf/echo ...). All are "shell syntax that cannot be statically
// analyzed", so Claude Code forces a manual-approval prompt. The analyzable
// rewrite — write the content to a file with the Write tool, then reference
// the file (< file, -F file, --body-file file) or use repeated -m flags —
// matches the user's allowlist and does not prompt.
//
// Use as a PreToolUse hook with matcher "Bash".
func handleNoInlineFile(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	if heredocRe.MatchString(bash.Command) || contentSubstRe.MatchString(bash.Command) {
		return nil, fmt.Errorf(
			"blocked: inlining content via a heredoc or $(cat/printf/echo ...) makes the " +
				"command unanalyzable, so Claude Code forces a manual-approval prompt. Write " +
				"the content to a file with the Write tool (Write to /tmp is allowlisted, no " +
				"prompt), then reference the file instead of inlining it:\n" +
				"  - data/query: `bq query --use_legacy_sql=false < /tmp/query.sql`\n" +
				"  - commit message: `git commit -F /tmp/msg.txt` (or repeated -m \"para\" flags)\n" +
				"  - PR body: `gh pr create --body-file /tmp/body.md`\n" +
				"These forms match the allowlist and run without prompting.")
	}
	return nil, nil
}

// handleNoShellLoops blocks Bash for/while/until loops. A loop makes the
// command unanalyzable, so Claude Code demands manual approval for it. The
// analyzable rewrite — running each iteration as its own command — matches the
// user's normal allowlist and needs no prompt.
//
// Use as a PreToolUse hook with matcher "Bash".
func handleNoShellLoops(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	for _, seg := range splitSegments(bash.Command) {
		switch cmd, _ := commandWord(strings.TrimSpace(seg)); cmd {
		case "for", "while", "until":
			return nil, fmt.Errorf(
				"blocked: shell %s loops make the command unanalyzable, so Claude Code "+
					"forces a manual-approval prompt. Run each iteration as its own command "+
					"(each will match the allowlist and won't prompt), or use the Read/Grep/Glob "+
					"tools to inspect files. For repeated metadata reads (e.g. bq show / bq ls "+
					"over several tables), issue them one per call.", cmd)
		}
	}
	return nil, nil
}

// handleNoInlinePython blocks inline `python[3] -c '<code>'` snippets. Inline
// interpreter code is unanalyzable (and usually text/JSON munging that a
// dedicated tool does without a prompt).
//
// Use as a PreToolUse hook with matcher "Bash".
func handleNoInlinePython(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	for _, seg := range splitSegments(bash.Command) {
		cmd, args := commandWord(strings.TrimSpace(seg))
		switch cmd {
		case "python", "python2", "python3":
			if hasAnyFlag(args, "-c") {
				return nil, fmt.Errorf(
					"blocked: inline `%s -c` snippets are unanalyzable and trigger a "+
						"manual-approval prompt. For JSON, pipe to `jq`; to read or search "+
						"files, use the Read/Grep tools; avoid shelling out to inline Python "+
						"for text munging.", cmd)
			}
		}
	}
	return nil, nil
}
