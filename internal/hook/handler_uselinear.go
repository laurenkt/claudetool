package hook

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func init() {
	Register("use-linear-mcp", handleUseLinearMCP)
}

const linearMCPMessage = "do not call Linear directly; use the Linear MCP server tools (mcp__linear-server__get_issue, etc.) instead"

// linearCLIRe matches the linear-cli command as a standalone token.
// `\b` alone is not enough because `-` is a non-word char, so `\blinear-cli\b`
// would match inside `my-linear-cli-helper`. We require the surrounding
// characters to be neither word chars nor hyphens.
var linearCLIRe = regexp.MustCompile(`(^|[^\w-])linear-cli([^\w-]|$)`)

// handleUseLinearMCP blocks attempts to reach Linear outside the MCP server:
// - Bash invocations of gh/curl/wget/linear-cli targeting Linear
// - WebFetch on linear.app URLs
// Use as a PreToolUse hook with matcher "Bash|WebFetch".
func handleUseLinearMCP(in *Input) (*Output, error) {
	switch in.ToolName {
	case "Bash":
		var bash BashInput
		if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
			return nil, nil
		}
		if looksLikeLinearCLI(strings.TrimSpace(bash.Command)) {
			return nil, fmt.Errorf("%s", linearMCPMessage)
		}
	case "WebFetch":
		var wf WebFetchInput
		if err := json.Unmarshal(in.ToolInput, &wf); err != nil {
			return nil, nil
		}
		if strings.Contains(strings.ToLower(wf.URL), "linear.app") {
			return nil, fmt.Errorf("%s", linearMCPMessage)
		}
	}
	return nil, nil
}

// looksLikeLinearCLI returns true if the Bash command appears to reach Linear
// via gh CLI, curl/wget, or the linear-cli binary.
func looksLikeLinearCLI(cmd string) bool {
	lower := strings.ToLower(cmd)
	if strings.Contains(lower, "gh issue") && strings.Contains(lower, "linear") {
		return true
	}
	if strings.Contains(lower, "gh api") && strings.Contains(lower, "linear") {
		return true
	}
	if (strings.Contains(lower, "curl") || strings.Contains(lower, "wget")) &&
		strings.Contains(lower, "linear.app") {
		return true
	}
	if linearCLIRe.MatchString(lower) {
		return true
	}
	return false
}
