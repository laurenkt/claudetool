package hook

import (
	"encoding/json"
	"fmt"
	"strings"
)

func init() {
	Register("no-command-substitution", handleNoCommandSubstitution)
}

// handleNoCommandSubstitution blocks command substitution — $(...) and
// backticks — which is unanalyzable and trips Claude Code's
// "command_substitution" / "simple_expansion" approval gates. The rewrite is
// always available: run the inner command as its own step and use its output
// explicitly. Arithmetic expansion $((...)) is allowed.
//
// Use as a PreToolUse hook with matcher "Bash".
func handleNoCommandSubstitution(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	if hasCommandSubstitution(bash.Command) {
		return nil, fmt.Errorf(
			"blocked: command substitution ($(...) or backticks) is unanalyzable and " +
				"triggers a manual-approval prompt. Run the inner command as its own step and " +
				"use its output explicitly. E.g. instead of " +
				"`docker inspect $(docker ps -q --filter …)`, run `docker ps -q --filter …` " +
				"first, then `docker inspect <ids>`. For file contents, write a file and use " +
				"`< file` / `-F file` rather than `$(cat …)`.")
	}
	return nil, nil
}

// hasCommandSubstitution reports whether the command contains $(...) or a
// backtick. Arithmetic expansion $((...)) is not command substitution and is
// allowed.
func hasCommandSubstitution(cmd string) bool {
	if strings.Contains(cmd, "`") {
		return true
	}
	for i := 0; i+1 < len(cmd); i++ {
		if cmd[i] == '$' && cmd[i+1] == '(' {
			if i+2 < len(cmd) && cmd[i+2] == '(' {
				continue // $(( arithmetic
			}
			return true
		}
	}
	return false
}
