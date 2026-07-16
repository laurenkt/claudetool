package hook

import (
	"encoding/json"
	"fmt"
	"regexp"
)

func init() {
	Register("no-git-c", handleNoGitC)
}

// gitDashCRe matches a `git -C <path>` invocation at a command position. The
// leading character class anchors `git` to the start of a statement (start of
// string or after a shell operator / opening paren) so a `git` mentioned mid-
// argument doesn't trip it. `-C` is case-sensitive: it is git's change-
// directory global option, distinct from `-c` (config).
var gitDashCRe = regexp.MustCompile("(?:^|[\\s|&;(`{])git\\s+-C\\b")

// handleNoGitC blocks `git -C <path>` invocations. Running git against another
// directory is unnecessary: the command already runs in the working directory,
// so plain `git ...` targets the right repository. Like `cd`, `-C` also trips
// Claude Code's approval gate for changing directory before git. Use as a
// PreToolUse hook with matcher "Bash".
func handleNoGitC(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	if gitDashCRe.MatchString(bash.Command) {
		return nil, fmt.Errorf(
			"blocked: `git -C <path>` is not needed. The command already runs " +
				"in the working directory, so run plain `git ...` (no `-C`, no " +
				"`cd`) to operate on this repository.")
	}
	return nil, nil
}
