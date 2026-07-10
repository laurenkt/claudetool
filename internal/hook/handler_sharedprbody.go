package hook

import (
	"encoding/json"
	"fmt"
	"regexp"
)

func init() {
	Register("no-shared-pr-body", handleNoSharedPRBody)
}

// sharedPRBodyRe matches the fixed `/tmp/pr-body.md` path agents reach for when
// drafting a PR body. The `\b` keeps it from matching longer names like
// `/tmp/pr-body.markdown`.
var sharedPRBodyRe = regexp.MustCompile(`/tmp/pr-body\.md\b`)

// handleNoSharedPRBody blocks Bash commands that read or write the shared
// `/tmp/pr-body.md` path. Concurrent agents all pick the same path, so one
// agent's `> /tmp/pr-body.md` clobbers another's in-flight PR body (or trips
// noclobber). Use as a PreToolUse hook with matcher "Bash".
func handleNoSharedPRBody(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	if sharedPRBodyRe.MatchString(bash.Command) {
		return nil, fmt.Errorf(
			"blocked: /tmp/pr-body.md is a fixed path shared by every agent, so concurrent " +
				"`gh pr create/edit` runs clobber each other's PR body. Use a unique file — " +
				"`body=$(mktemp); ...write... > \"$body\"; gh pr create --body-file \"$body\"` — " +
				"or skip the file and pipe the body via stdin with `--body-file -`",
		)
	}
	return nil, nil
}
