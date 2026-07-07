package hook

import (
	"encoding/json"
	"strings"
)

func init() {
	Register("auto-approve-readonly", handleAutoApproveReadonly)
}

// handleAutoApproveReadonly auto-approves Bash commands that are composed
// entirely of known read-only operations, so Claude Code's manual-approval
// gate (e.g. "compound command contains cd with output redirection") never
// fires for harmless exploration like `cd <dir> && find ... | grep ...`.
//
// Use as a PreToolUse hook with matcher "Bash". It returns an "allow"
// permission decision only when every segment of the command is on the
// read-only allowlist and no mutating construct is present; otherwise it
// stays silent (returns nil) and the normal permission flow applies. It never
// blocks — it can only turn a prompt into an auto-approval, never the reverse.
func handleAutoApproveReadonly(in *Input) (*Output, error) {
	if in.HookEventName != "PreToolUse" || in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	if !isReadOnlyCommand(bash.Command) {
		return nil, nil
	}

	return &Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "allow",
			PermissionDecisionReason: "read-only command (auto-approve-readonly): every segment is on the read-only allowlist",
		},
	}, nil
}

// readOnlyCmds are commands that cannot mutate the filesystem or external
// state on their own. Commands with both read and write forms (git, gh,
// find, sort, execute-in-data-shell) are handled separately below.
var readOnlyCmds = map[string]bool{
	"cd": true, "pwd": true, "echo": true, "printf": true, "true": true, "false": true,
	"ls": true, "cat": true, "tac": true, "head": true, "tail": true, "wc": true,
	"grep": true, "egrep": true, "fgrep": true, "rg": true,
	"sort": true, "uniq": true, "cut": true, "tr": true, "column": true, "nl": true, "rev": true,
	"diff": true, "comm": true, "paste": true, "join": true, "fold": true,
	"expand": true, "unexpand": true, "look": true,
	"jq": true, "yq": true,
	"dirname": true, "basename": true, "realpath": true, "readlink": true,
	"stat": true, "file": true, "tree": true, "du": true, "df": true,
	"date": true, "which": true, "type": true, "env": true, "printenv": true,
	"test": true,
	// Hashing / dumping / misc read-only utilities (from usage sweep).
	"md5": true, "md5sum": true, "shasum": true, "sha1sum": true,
	"sha256sum": true, "cksum": true, "xxd": true, "od": true,
	"hexdump": true, "strings": true, "seq": true, "ps": true, "sleep": true,
}

// gitReadSubcmds are git subcommands that only read repository state.
// branch/config/worktree/stash/tag have both read and write forms and are
// handled with flag guards in gitIsReadOnly.
var gitReadSubcmds = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true,
	"rev-parse": true, "describe": true, "ls-files": true, "ls-tree": true,
	"cat-file": true, "blame": true, "shortlog": true, "reflog": true,
	"whatchanged": true, "grep": true, "merge-base": true, "rev-list": true,
}

// gitMutatingBranchFlags mutate refs when passed to `git branch`.
var gitMutatingBranchFlags = map[string]bool{
	"-d": true, "-D": true, "--delete": true, "-m": true, "-M": true,
	"--move": true, "-c": true, "-C": true, "--copy": true,
	"-u": true, "--set-upstream-to": true, "--unset-upstream": true,
	"--edit-description": true,
}

// gitIsReadOnly reports whether a git invocation only reads. Pure-read
// subcommands are allowed outright; branch/stash/config/worktree/tag are
// allowed only in their listing/reading forms.
func gitIsReadOnly(args []string) bool {
	if len(args) == 0 {
		return false
	}
	sub := unquote(args[0])
	rest := args[1:]
	if gitReadSubcmds[sub] {
		return true
	}
	switch sub {
	case "branch":
		// Listing only: no mutating flag, and no bare branch-name positional
		// (which would create/rename) unless it's an explicit --list pattern.
		for _, a := range rest {
			a = unquote(a)
			if gitMutatingBranchFlags[a] {
				return false
			}
			if !strings.HasPrefix(a, "-") && !hasAnyFlag(rest, "-l", "--list") {
				return false
			}
		}
		return true
	case "stash":
		return len(rest) > 0 && (unquote(rest[0]) == "list" || unquote(rest[0]) == "show")
	case "worktree":
		return len(rest) > 0 && unquote(rest[0]) == "list"
	case "config":
		return hasAnyFlag(rest, "--get", "--get-all", "--get-regexp", "--list", "-l")
	case "tag":
		// Bare `git tag` and `-l/--list` list tags; a positional creates one.
		for _, a := range rest {
			if !strings.HasPrefix(unquote(a), "-") && !hasAnyFlag(rest, "-l", "--list") {
				return false
			}
		}
		return true
	}
	return false
}

// ghReadSubcmds maps a gh top-level command to the read-only subcommands
// allowed under it.
var ghReadSubcmds = map[string]map[string]bool{
	"pr":       {"view": true, "checks": true, "list": true, "diff": true, "status": true},
	"issue":    {"view": true, "list": true, "status": true},
	"run":      {"view": true, "list": true},
	"workflow": {"view": true, "list": true},
	"repo":     {"view": true},
}

// adbtReadSubcmds are execute-in-data-shell adbt subcommands that read
// metadata only (no query execution, so no BigQuery scan cost).
var adbtReadSubcmds = map[string]bool{
	"dataset": true, "ls": true, "list": true, "show": true,
	"describe": true, "status": true, "debug": true, "compile": true,
	"upstream": true, "parse": true,
}

// isReadOnlyCommand reports whether every segment of a shell command is a
// known read-only operation. It errs toward false (prompt) whenever it is
// unsure: unrecognized commands, mutating flags, redirections to real files,
// unbalanced quotes, and command/process substitution all cause it to return
// false.
func isReadOnlyCommand(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}

	// Substitutions run nested commands the classifier cannot see. Checked on
	// the raw string (conservative): a literal $(...) or backtick inside a
	// quoted jq/awk program also declines, which only costs an extra prompt.
	// Doing it here — rather than on the quote-masked string — is what keeps a
	// real "$(rm ...)" inside double quotes from being silently approved.
	if strings.Contains(command, "$(") || strings.Contains(command, "`") ||
		strings.Contains(command, "<(") || strings.Contains(command, ">(") {
		return false
	}

	// Mask the contents of quoted spans so shell operators and redirections
	// that live inside quotes (e.g. the `|` in a jq program `'a | select(b)'`)
	// are not mistaken for real ones. Unbalanced quotes -> decline.
	masked, balanced := maskQuotes(command)
	if !balanced {
		return false
	}

	// Any redirection to something other than /dev/null (or a dup like 2>&1)
	// can write a file. Reject unless every redirect target is safe.
	if !redirectsAreSafe(masked) {
		return false
	}

	for _, seg := range splitOnMask(command, masked) {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if !segmentIsReadOnly(seg) {
			return false
		}
	}
	return true
}

// maskQuotes returns a copy of cmd with the CONTENTS of single- and
// double-quoted spans replaced by 'x', so operators/redirections inside quotes
// are not read as real. Quote delimiters are kept. The bool is false if a
// quote is left unclosed.
func maskQuotes(cmd string) (string, bool) {
	out := []byte(cmd)
	var q byte
	for i := 0; i < len(out); i++ {
		c := out[i]
		if q != 0 {
			if c == q {
				q = 0
			} else {
				out[i] = 'x'
			}
			continue
		}
		if c == '\'' || c == '"' {
			q = c
		}
	}
	return string(out), q == 0
}

// splitSegments splits a command into segments on the control operators
// && || ; | (and newlines), respecting quotes so an operator inside a quoted
// string does not split.
func splitSegments(command string) []string {
	masked, _ := maskQuotes(command)
	return splitOnMask(command, masked)
}

// splitOnMask finds operator boundaries in masked (where quoted spans are
// neutralized) and slices the aligned original string at those boundaries.
func splitOnMask(orig, masked string) []string {
	var segs []string
	start := 0
	for i := 0; i < len(masked); {
		switch {
		case masked[i] == '\n' || masked[i] == ';':
			segs = append(segs, orig[start:i])
			i++
			start = i
		case masked[i] == '&' && i+1 < len(masked) && masked[i+1] == '&':
			segs = append(segs, orig[start:i])
			i += 2
			start = i
		case masked[i] == '|' && i+1 < len(masked) && masked[i+1] == '|':
			segs = append(segs, orig[start:i])
			i += 2
			start = i
		case masked[i] == '|':
			segs = append(segs, orig[start:i])
			i++
			start = i
		default:
			i++
		}
	}
	segs = append(segs, orig[start:])
	return segs
}

// commandWord returns the effective command name of a single segment (no
// control operators) and its argument tokens, skipping leading VAR=value
// environment assignments and stripping any leading path (/usr/bin/grep ->
// grep, ./x -> x). Returns "" when the segment has no command word.
func commandWord(seg string) (string, []string) {
	fields := strings.Fields(seg)
	for len(fields) > 0 && isEnvAssignment(fields[0]) {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return "", nil
	}
	cmd := unquote(fields[0])
	if i := strings.LastIndex(cmd, "/"); i >= 0 {
		cmd = cmd[i+1:]
	}
	return cmd, fields[1:]
}

// segmentIsReadOnly reports whether a single command segment (no control
// operators) is a read-only operation.
func segmentIsReadOnly(seg string) bool {
	cmd, args := commandWord(seg)
	// Reject empty words and redirection tokens masquerading as the command.
	if cmd == "" || strings.ContainsAny(cmd, "<>&") {
		return false
	}

	switch cmd {
	case "find":
		return findIsReadOnly(args)
	case "sort":
		return !hasAnyFlag(args, "-o", "--output")
	case "base64":
		return !hasAnyFlag(args, "-o", "--output")
	case "git":
		return gitIsReadOnly(args)
	case "gh":
		return ghIsReadOnly(args)
	case "bq":
		return bqIsReadOnly(args)
	case "execute-in-data-shell":
		return dataShellIsReadOnly(args)
	case "adbt", "model-routing", "run-data-standards", "modelgen":
		// Same Monzo data tools, invoked bare (not wrapped in the data shell).
		return dataToolIsReadOnly(cmd, args)
	default:
		return readOnlyCmds[cmd]
	}
}

// findIsReadOnly rejects find invocations that can execute or write:
// -delete, -exec/-execdir/-ok/-okdir, and -fprint*/-fls file-writing actions.
func findIsReadOnly(args []string) bool {
	for _, a := range args {
		switch a {
		case "-delete", "-exec", "-execdir", "-ok", "-okdir",
			"-fprint", "-fprintf", "-fprint0", "-fls":
			return false
		}
	}
	return true
}

// bqReadSubcmds are `bq` subcommands that only read metadata or a tiny row
// sample — no query job is run, so there is no BigQuery scan cost. `bq query`
// is deliberately excluded (it scans, and scanning costs money).
var bqReadSubcmds = map[string]bool{
	"show": true, "ls": true, "head": true, "version": true,
}

// bqIsReadOnly reports whether a bq invocation only reads metadata. It finds
// the first non-flag token (the subcommand), skipping global flags like
// --format=prettyjson that may precede it.
func bqIsReadOnly(args []string) bool {
	for _, a := range args {
		a = unquote(a)
		if a == "--version" || a == "-version" {
			return true
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return bqReadSubcmds[a]
	}
	return false
}

func ghIsReadOnly(args []string) bool {
	if len(args) == 0 {
		return false
	}
	top := unquote(args[0])
	// `gh api` is read-only only when it stays a GET with no body fields.
	if top == "api" {
		return !hasAnyFlag(args, "-X", "--method", "-f", "--field",
			"-F", "--raw-field", "--input")
	}
	subs, ok := ghReadSubcmds[top]
	return ok && len(args) > 1 && subs[unquote(args[1])]
}

// dataToolIsReadOnly reports whether a Monzo data tool invocation only reads.
// Query execution / builds / mutations are not auto-approved. Used both for
// the bare tool and for `execute-in-data-shell <tool> ...`.
func dataToolIsReadOnly(tool string, rest []string) bool {
	switch tool {
	case "adbt":
		return len(rest) > 0 && adbtReadSubcmds[unquote(rest[0])]
	case "run-data-standards":
		return (len(rest) > 0 && unquote(rest[0]) == "static") || hasAnyFlag(rest, "--check")
	case "model-routing":
		return len(rest) > 0 && strings.HasPrefix(unquote(rest[0]), "get")
	case "modelgen":
		return (len(rest) > 0 && unquote(rest[0]) == "version") || hasAnyFlag(rest, "--version")
	}
	return false
}

// dataShellIsReadOnly reports whether an `execute-in-data-shell <tool> ...`
// invocation only reads. Query execution / builds / `bash` (arbitrary) are not
// auto-approved.
func dataShellIsReadOnly(args []string) bool {
	if len(args) == 0 {
		return false
	}
	tool := unquote(args[0])
	rest := args[1:]
	switch tool {
	case "which", "ps", "cat", "ls", "echo", "pwd", "head", "tail", "grep":
		return true
	}
	return dataToolIsReadOnly(tool, rest)
}

// redirectsAreSafe reports whether every > or >> redirection targets a safe
// sink (/dev/null) or is a descriptor dup (2>&1, >&2). Any redirect to a real
// file makes the command a writer, so it must prompt.
func redirectsAreSafe(command string) bool {
	for i := 0; i < len(command); i++ {
		if command[i] != '>' {
			continue
		}
		// Skip past >> and a leading fd digit was already consumed as text.
		j := i + 1
		if j < len(command) && command[j] == '>' {
			j++
		}
		// Descriptor dup: >&1, >&2.
		if j < len(command) && command[j] == '&' {
			i = j
			continue
		}
		for j < len(command) && (command[j] == ' ' || command[j] == '\t') {
			j++
		}
		rest := command[j:]
		if !strings.HasPrefix(rest, "/dev/null") {
			return false
		}
		i = j
	}
	return true
}

func isEnvAssignment(tok string) bool {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	for _, r := range tok[:eq] {
		if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func hasAnyFlag(args []string, flags ...string) bool {
	for _, a := range args {
		a = unquote(a)
		for _, f := range flags {
			if a == f || strings.HasPrefix(a, f+"=") {
				return true
			}
		}
	}
	return false
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
