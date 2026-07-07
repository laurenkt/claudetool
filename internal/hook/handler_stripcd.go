package hook

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

func init() {
	Register("strip-redundant-cd", handleStripRedundantCd)
}

// handleStripRedundantCd removes a leading `cd <dir> &&` (or `; ` / newline)
// when <dir> is already the command's working directory, so the cd is a
// guaranteed no-op. Such a redundant cd turns an otherwise-analyzable command
// into a compound that trips Claude Code's approval gates (e.g. "changes
// directory before running git, which can execute untrusted hooks"). Removing
// it lets the real command match the allowlist and run without a prompt.
//
// Use as a PreToolUse hook with matcher "Bash". It rewrites the command via
// updatedInput; it never blocks, and only ever strips a cd that does nothing.
func handleStripRedundantCd(in *Input) (*Output, error) {
	if in.ToolName != "Bash" || in.CWD == "" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	stripped, ok := stripRedundantCd(bash.Command, in.CWD)
	if !ok {
		return nil, nil
	}

	updated := map[string]any{"command": stripped}
	if bash.Description != "" {
		updated["description"] = bash.Description
	}
	return &Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName: "PreToolUse",
			UpdatedInput:  updated,
		},
	}, nil
}

// stripRedundantCd returns the command with a leading `cd <cwd>` and its
// following separator removed, and true, when the cd target resolves to cwd
// (making it a no-op) and is followed by another command. Otherwise it returns
// "", false.
func stripRedundantCd(command, cwd string) (string, bool) {
	trimmed := strings.TrimLeft(command, " \t")
	if !strings.HasPrefix(trimmed, "cd ") {
		return "", false
	}
	rest := trimmed[len("cd "):]

	// Find where the cd statement ends (&&, ; or newline), respecting quotes.
	sepIdx, sepLen := firstTopLevelSep(rest)
	if sepIdx < 0 {
		return "", false
	}

	dir := unquote(strings.TrimSpace(rest[:sepIdx]))
	if dir == "" || filepath.Clean(dir) != filepath.Clean(cwd) {
		return "", false
	}

	remainder := strings.TrimLeft(rest[sepIdx+sepLen:], " \t\n")
	if remainder == "" {
		return "", false
	}
	return remainder, true
}

// firstTopLevelSep returns the index and length of the first &&, ; or newline
// in s that is not inside quotes. Returns -1, 0 if none.
func firstTopLevelSep(s string) (int, int) {
	masked, _ := maskQuotes(s)
	for i := 0; i < len(masked); i++ {
		switch {
		case masked[i] == '\n' || masked[i] == ';':
			return i, 1
		case masked[i] == '&' && i+1 < len(masked) && masked[i+1] == '&':
			return i, 2
		}
	}
	return -1, 0
}
