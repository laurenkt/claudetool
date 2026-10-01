package hook

import (
	"encoding/json"
	"fmt"
)

func init() {
	Register("no-avoidable-cd", handleNoAvoidableCD)
}

// cdPassengers are commands whose behaviour does not depend on the working
// directory, so their presence alongside git/gh does not justify a cd.
var cdPassengers = map[string]bool{
	"echo": true, "printf": true, "pwd": true, "true": true, "false": true, ":": true,
}

// handleNoAvoidableCD blocks a Bash command that changes directory purely to
// run git/gh, nudging the agent to target the repo directly (git -C <dir>,
// gh ... -R <owner/repo>) instead of cd-ing into it. It never approves — it
// only blocks — and it stays silent the moment any segment might genuinely
// need the working directory (scripts, dbt/adbt, make, relative paths), so
// legitimate cd use is untouched.
//
// Motivation: a leading `cd <repo> && git ...` trips Claude Code's built-in
// "changes directory before running git (untrusted hooks)" gate, whereas the
// equivalent `git -C <repo> ...` matches a `Bash(git *)` allow and runs with
// no prompt at all.
func handleNoAvoidableCD(in *Input) (*Output, error) {
	if in.HookEventName != "PreToolUse" || in.ToolName != "Bash" {
		return nil, nil
	}
	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}
	if !cdIsAvoidable(bash.Command) {
		return nil, nil
	}
	return nil, fmt.Errorf("blocked: avoidable 'cd' before git/gh. Drop the cd and target the repo directly — 'git -C <dir> ...' and 'gh ... -R <owner/repo>'. A leading cd also trips Claude Code's cd-before-git untrusted-hooks gate, while 'git -C' matches your git allow and runs with no prompt. (cd is fine when a later step genuinely needs the working directory: a script, dbt/adbt, make, or a relative path.)")
}

// cdIsAvoidable reports whether a command changes directory solely to run
// git/gh (with only inert passengers like echo alongside). It requires at
// least one git/gh segment, and returns false (cd allowed) the moment any
// segment could depend on the working directory.
func cdIsAvoidable(command string) bool {
	sawCD := false
	sawGitGh := false
	for _, seg := range splitSegments(command) {
		cmd, _ := commandWord(seg)
		switch {
		case cmd == "":
			// empty segment or a bare VAR=value assignment — inert
		case cmd == "cd":
			sawCD = true
		case cmd == "git" || cmd == "gh":
			sawGitGh = true
		case cdPassengers[cmd]:
			// inert passenger; does not need the working directory
		default:
			return false // something that might need the working directory
		}
	}
	return sawCD && sawGitGh
}
