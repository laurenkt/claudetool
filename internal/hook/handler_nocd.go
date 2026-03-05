package hook

import (
	"encoding/json"
	"fmt"
	"strings"
)

func init() {
	Register("no-cd", handleNoCD)
}

// handleNoCD blocks Bash commands that change directory.
// Use as a PreToolUse hook with matcher "Bash".
func handleNoCD(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	cmd := strings.TrimSpace(bash.Command)
	if hasCD(cmd) {
		return nil, fmt.Errorf("blocked: cd commands are not allowed; work in the current directory")
	}
	return nil, nil
}

// hasCD checks whether a shell command string contains a cd invocation.
func hasCD(cmd string) bool {
	// Check each line/statement for cd at the start or after && / || / ; / |
	for _, line := range strings.Split(cmd, "\n") {
		line = strings.TrimSpace(line)
		if line == "cd" || strings.HasPrefix(line, "cd ") || strings.HasPrefix(line, "cd\t") {
			return true
		}
		// Check for cd after shell operators
		for _, sep := range []string{"&&", "||", ";", "|"} {
			parts := strings.Split(line, sep)
			for _, part := range parts[1:] {
				part = strings.TrimSpace(part)
				if part == "cd" || strings.HasPrefix(part, "cd ") || strings.HasPrefix(part, "cd\t") {
					return true
				}
			}
		}
	}
	return false
}
