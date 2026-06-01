package hook

import (
	"encoding/json"
	"fmt"
	"regexp"
)

func init() {
	Register("no-diff-master", handleNoDiffMaster)
}

var diffMasterRe = regexp.MustCompile(`git\s+diff\b.*\b(?:origin/)?(?:master|main)\.\.\.\S*`)

// handleNoDiffMaster blocks git diff master...HEAD and similar commands.
// In large monorepos, the output is enormous and not useful.
func handleNoDiffMaster(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	if diffMasterRe.MatchString(bash.Command) {
		return nil, fmt.Errorf(
			"blocked: git diff against master/main...HEAD is not useful in a large monorepo — " +
				"the diff will be enormous. Instead, look at specific commits on the branch " +
				"(git log master..HEAD), or diff specific files (git diff HEAD~1 -- path/to/file)",
		)
	}
	return nil, nil
}
