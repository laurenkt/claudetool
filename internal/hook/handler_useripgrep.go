package hook

import (
	"encoding/json"
	"fmt"
	"regexp"
)

func init() {
	Register("use-ripgrep", handleUseRipgrep)
}

// grepRe matches a grep/egrep/fgrep invocation at a command position. The
// leading character class anchors the match to the start of a statement
// (start of string or after a shell operator / opening paren) so a longer
// tool name ending in "grep" (ripgrep, zgrep, pgrep) doesn't trip it.
var grepRe = regexp.MustCompile("(?:^|[\\s|&;(`{])(?:grep|egrep|fgrep)\\b")

// handleUseRipgrep blocks grep/egrep/fgrep invocations in favor of ripgrep
// (rg): rg is faster, recurses by default, and respects .gitignore so it
// skips build artifacts and vendored code without extra flags. Use as a
// PreToolUse hook with matcher "Bash".
func handleUseRipgrep(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	if grepRe.MatchString(bash.Command) {
		return nil, fmt.Errorf(
			"blocked: use ripgrep (`rg`) instead of grep/egrep/fgrep. rg " +
				"recurses and respects .gitignore by default, so `rg pattern` " +
				"replaces `grep -r pattern .`, and `rg pattern file` replaces " +
				"`grep pattern file`.")
	}
	return nil, nil
}
