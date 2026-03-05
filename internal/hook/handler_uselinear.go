package hook

import (
	"encoding/json"
	"fmt"
	"strings"
)

func init() {
	Register("use-linear-mcp", handleUseLinearMCP)
}

// handleUseLinearMCP blocks gh CLI calls that try to look up Linear tickets.
// Use as a PreToolUse hook with matcher "Bash".
func handleUseLinearMCP(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	cmd := strings.TrimSpace(bash.Command)
	if looksLikeLinearGH(cmd) {
		return nil, fmt.Errorf("do not use gh CLI for Linear tickets; use the Linear MCP server tools (mcp__linear-server__get_issue, etc.) instead")
	}
	return nil, nil
}

// looksLikeLinearGH returns true if the command appears to use gh CLI
// to look up or interact with Linear issues.
func looksLikeLinearGH(cmd string) bool {
	lower := strings.ToLower(cmd)
	// Catch "gh issue view" referencing linear
	if strings.Contains(lower, "gh issue") && strings.Contains(lower, "linear") {
		return true
	}
	// Catch gh api calls to linear
	if strings.Contains(lower, "gh api") && strings.Contains(lower, "linear") {
		return true
	}
	// Catch curl to linear.app
	if (strings.Contains(lower, "curl") || strings.Contains(lower, "wget")) &&
		strings.Contains(lower, "linear.app") {
		return true
	}
	return false
}
