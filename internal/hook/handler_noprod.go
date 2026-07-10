package hook

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func init() {
	Register("no-prod", handleNoProd)
}

// prodEnvFlagRe matches a production environment flag. The trailing `\b` keeps
// `prod` from matching longer values (e.g. `prodigy`), while the alternation
// still catches `production`.
var prodEnvFlagRe = regexp.MustCompile(`(?:^|\s)(?:-e|--environment)(?:\s+|=)(?:prod|production)\b`)

// poundTokenRe matches the `£` binary in a command position. The preceding
// character class excludes quotes, so a quoted mention like `echo "£ -e prod"`
// is left alone rather than blocked.
var poundTokenRe = regexp.MustCompile("(?:^|[\\s(`{])£(?:\\s|$)")

// handleNoProd blocks invocations of the Monzo `£` CLI that target the
// production environment (`£ -e prod ...`). Agents must never run operations
// directly against prod; lower environments like `£ -e s101 "..."` are fine.
// Use as a PreToolUse hook with matcher "Bash".
func handleNoProd(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	if hasProdCLICall(bash.Command) {
		return nil, fmt.Errorf(
			"blocked: agents are STRICTLY forbidden from running operations directly on the " +
				"production environment (`£ -e prod ...`). Ask the human operator to run this " +
				"command against prod and paste the results back to you using the AskUserQuestion tool. " +
				"Lower environments (e.g. `£ -e s101 \"...\"`) are fine.",
		)
	}
	return nil, nil
}

// hasProdCLICall reports whether any statement in a shell command invokes the
// `£` CLI with a production environment flag. It splits on shell operators so a
// prod flag on an unrelated command in the same line doesn't trip a `£` call
// (and vice versa).
func hasProdCLICall(cmd string) bool {
	for _, seg := range splitShellSegments(cmd) {
		if poundTokenRe.MatchString(seg) && prodEnvFlagRe.MatchString(seg) {
			return true
		}
	}
	return false
}

// splitShellSegments breaks a command string into individual statements on
// newlines and shell operators.
func splitShellSegments(cmd string) []string {
	segs := []string{cmd}
	for _, sep := range []string{"\n", "&&", "||", ";", "|"} {
		var next []string
		for _, s := range segs {
			next = append(next, strings.Split(s, sep)...)
		}
		segs = next
	}
	return segs
}
