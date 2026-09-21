package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// asyncLogOverride redirects the diagnostic log when non-empty. Tests set it;
// production leaves it blank and uses asyncLogPath's default.
var asyncLogOverride string

// asyncLogPath is the fixed, easily-tailed file async reviewers append to, so a
// user can watch whether reviews actually run and why they stay silent:
//
//	tail -f ~/.claude/claudetool-async.log
func asyncLogPath() string {
	if asyncLogOverride != "" {
		return asyncLogOverride
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".claude", "claudetool-async.log")
	}
	return filepath.Join(os.TempDir(), "claudetool-async.log")
}

// logAsync appends one tab-separated diagnostic line: time, handler, outcome,
// file. Best-effort — any open/write failure is ignored, since a reviewer's
// diagnostics must never change hook behaviour. Newlines in outcome (e.g. a
// multi-line error) are flattened to keep one event per line.
func logAsync(handler, outcome, filePath string) {
	f, err := os.OpenFile(asyncLogPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	outcome = strings.ReplaceAll(outcome, "\n", " ")
	fmt.Fprintf(f, "%s\t%s\t%s\t%s\n", time.Now().Format(time.RFC3339), handler, outcome, filePath)
}

// ReviewTier selects which model the background reviewer runs on, trading
// latency and cost against judgement quality.
type ReviewTier int

const (
	FastCheap      ReviewTier = iota // haiku
	MediumBalanced                   // sonnet (default)
	SlowAccurate                     // opus
)

// model returns the `claude --model` alias for the tier.
func (t ReviewTier) model() string {
	switch t {
	case FastCheap:
		return "haiku"
	case SlowAccurate:
		return "opus"
	default:
		return "sonnet"
	}
}

// parseTier maps a -tier flag value to a ReviewTier, falling back to def.
func parseTier(v string, def ReviewTier) ReviewTier {
	switch strings.ToLower(v) {
	case "fast", "fastcheap", "haiku":
		return FastCheap
	case "medium", "balanced", "mediumbalanced", "sonnet":
		return MediumBalanced
	case "slow", "accurate", "slowaccurate", "opus":
		return SlowAccurate
	default:
		return def
	}
}

// tierFromArgs scans the flag tail for `-tier <value>` or `-tier=<value>`.
// Since an async-review handler runs as its own dedicated hook command, the
// flag tail applies only to it.
func tierFromArgs(args []string, def ReviewTier) ReviewTier {
	for i, a := range args {
		switch {
		case a == "-tier" || a == "--tier":
			if i+1 < len(args) {
				return parseTier(args[i+1], def)
			}
		case strings.HasPrefix(a, "-tier="):
			return parseTier(strings.TrimPrefix(a, "-tier="), def)
		case strings.HasPrefix(a, "--tier="):
			return parseTier(strings.TrimPrefix(a, "--tier="), def)
		}
	}
	return def
}

// asyncReview describes a check that dispatches a code change to a headless
// `claude` instance for evaluation. It is meant to be configured as a
// PostToolUse (Write|Edit) hook with `asyncRewake: true`: the handler runs in
// the background without blocking the working agent, and only wakes it (via
// exit 2 + stderr) when the reviewer returns findings.
type asyncReview struct {
	name         string                 // handler name, e.g. "valuable-comments"
	fileSuffixes []string               // required file suffixes, any one matches (nil = any file)
	skipSuffix   string                 // optional suffix to skip, e.g. "_test.go" ("" = none)
	tier         ReviewTier             // default model tier
	precheck     func(text string) bool // cheap gate: only dispatch when true (nil = always)
	rubric       string                 // review criteria embedded in the reviewer prompt
	summary      string                 // header line prepended to REVISE feedback

	// review runs the reviewer and returns its raw output. Injected in tests;
	// nil falls back to runClaudeReview against the real `claude` binary.
	review func(prompt, model string) (string, error)
}

// handler returns a Handler closure for use with Register.
func (a asyncReview) handler() Handler {
	return func(in *Input) (*Output, error) {
		switch in.ToolName {
		case "Write", "Edit", "MultiEdit":
		default:
			return nil, nil
		}

		filePath, text := changedContent(in)
		if filePath == "" {
			return nil, nil
		}
		if !matchesSuffix(filePath, a.fileSuffixes) {
			return nil, nil
		}
		if a.skipSuffix != "" && strings.HasSuffix(filePath, a.skipSuffix) {
			return nil, nil
		}
		if a.precheck != nil && !a.precheck(text) {
			return nil, nil
		}

		tier := tierFromArgs(in.Args, a.tier)

		review := a.review
		if review == nil {
			review = runClaudeReview
		}

		logAsync(a.name, "DISPATCH", filePath)

		out, err := review(buildReviewPrompt(a.rubric, filePath, text), tier.model())
		if err != nil {
			// Reviewer unavailable (claude not on PATH, not logged in, timeout,
			// etc.). We still never wake the agent on an infrastructure failure,
			// but we record it: a swallowed error here is indistinguishable from
			// a genuine PASS, so without this line a broken reviewer looks like a
			// clean bill of health.
			logAsync(a.name, "ERROR: "+err.Error(), filePath)
			return nil, nil
		}

		verdict, feedback := parseVerdict(out)
		if verdict != "REVISE" {
			logAsync(a.name, "PASS", filePath)
			return nil, nil
		}

		logAsync(a.name, "REVISE", filePath)

		// exit 2 with the feedback on stderr; under asyncRewake this is what
		// surfaces back to the working agent.
		return nil, fmt.Errorf("%s\n\n%s", a.summary, feedback)
	}
}

// matchesSuffix reports whether filePath ends in one of suffixes. An empty
// list means the check is not scoped to a language and accepts any file.
func matchesSuffix(filePath string, suffixes []string) bool {
	if len(suffixes) == 0 {
		return true
	}
	for _, s := range suffixes {
		if strings.HasSuffix(filePath, s) {
			return true
		}
	}
	return false
}

// changedContent extracts the file path and the newly written/edited text from
// a Write, Edit, or MultiEdit tool input.
func changedContent(in *Input) (filePath, text string) {
	switch in.ToolName {
	case "Write":
		var w WriteInput
		if err := json.Unmarshal(in.ToolInput, &w); err != nil {
			return "", ""
		}
		return w.FilePath, w.Content
	case "Edit":
		var e EditInput
		if err := json.Unmarshal(in.ToolInput, &e); err != nil {
			return "", ""
		}
		return e.FilePath, e.NewString
	case "MultiEdit":
		var m MultiEditInput
		if err := json.Unmarshal(in.ToolInput, &m); err != nil {
			return "", ""
		}
		// Join every edit's new text so the reviewer sees all the changes at once.
		var b strings.Builder
		for _, e := range m.Edits {
			b.WriteString(e.NewString)
			b.WriteString("\n")
		}
		return m.FilePath, b.String()
	}
	return "", ""
}

// parseVerdict reads the reviewer's protocol output. The first non-empty line
// is the verdict token (PASS or REVISE); the remainder is the feedback body.
// Anything unrecognised is treated as PASS so a confused reviewer can't wake
// the agent with noise.
func parseVerdict(out string) (verdict, feedback string) {
	lines := strings.Split(out, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return "PASS", ""
	}
	first := strings.TrimSpace(lines[i])
	switch strings.ToUpper(first) {
	case "REVISE":
		body := strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
		if body == "" {
			body = "The reviewer flagged an issue but gave no detail."
		}
		return "REVISE", body
	default:
		return "PASS", ""
	}
}

// Subject-agnostic so each check can scope it through its own rubric.
const reviewerSystemPrompt = `You are a code reviewer. You are given a rubric and a snippet of newly written or edited code. Judge the snippet ONLY against the rubric — ignore anything the rubric does not ask about. Follow the output protocol exactly: no preamble, no markdown fences, nothing else.`

// buildReviewPrompt assembles the prompt piped to `claude -p` on stdin.
func buildReviewPrompt(rubric, filePath, text string) string {
	return fmt.Sprintf(`%s

OUTPUT PROTOCOL
If the changed code below is fine by the rubric above, respond with exactly:
PASS

Otherwise respond with:
REVISE
- <one short bullet per issue: what is wrong -> what to do>
(nothing after the list)

FILE: %s
--- BEGIN CHANGED CODE ---
%s
--- END CHANGED CODE ---
`, rubric, filePath, text)
}

// runClaudeReview dispatches the prompt to a headless `claude` instance.
//
//   - `--settings "{}"` stops the inner claude from inheriting the user's hooks,
//     which avoids recursion and shaves startup latency.
//   - `--strict-mcp-config` with no `--mcp-config` loads zero MCP servers,
//     ignoring the user's project/global/plugin MCP configs. Without it the
//     reviewer boots every configured MCP server on startup; any one with a
//     stale token triggers an interactive auth flow that a headless (no-TTY)
//     claude can't complete, so it exits 1 with no output. That was the
//     intermittent failure that let unreviewed code through. We do NOT use
//     `--bare`: it forces auth to ANTHROPIC_API_KEY only, never reading the
//     OAuth/keychain credentials the reviewer relies on.
//   - `--disallowed-tools` keeps the reviewer to pure text analysis; it never
//     needs to touch the filesystem or run commands.
//   - cmd.Env drops the host's CLAUDE_CODE_* / CLAUDECODE vars — see
//     reviewerEnv for why.
func runClaudeReview(prompt, model string) (string, error) {
	cmd := exec.Command("claude",
		"-p",
		"--model", model,
		"--settings", "{}",
		"--strict-mcp-config",
		"--disallowed-tools", "Bash Edit Write Read Glob Grep WebFetch WebSearch",
		"--append-system-prompt", reviewerSystemPrompt,
	)
	cmd.Env = reviewerEnv()
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.Output()
	if err != nil {
		// "exit status 1" alone can't tell a login failure from a rate limit from
		// a bad flag. claude may report on either stream — stderr lands in
		// ExitError.Stderr, but errors like "Not logged in" print to stdout, which
		// Output returns in out. Surface whichever we got.
		detail := strings.TrimSpace(string(out))
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		if detail != "" {
			return "", fmt.Errorf("%w: %s", err, detail)
		}
		return "", err
	}
	return string(out), nil
}

// reviewerEnv returns the process environment with the host Claude Code
// session's markers removed: everything prefixed CLAUDE_CODE_ plus CLAUDECODE.
//
// When claudetool runs as a hook, the host exports child-session/auth-broker
// vars — CLAUDE_CODE_CHILD_SESSION, CLAUDE_CODE_MESSAGING_SOCKET/TOKEN,
// CLAUDE_CODE_SDK_HAS_{HOST_AUTH,OAUTH}_REFRESH, and CLAUDECODE=1. A freshly
// spawned `claude -p` that inherits them believes it is a child session that
// must fetch auth from the host broker; but it is a new top-level process, can't
// reach the broker, and exits "Not logged in" — silently swallowed, so
// unreviewed code sailed through. A plain terminal carries none of these, which
// is why the same command authenticates there. Stripping them lets the reviewer
// authenticate through the normal keychain/OAuth path. CLAUDE_CONFIG_DIR and
// other non-CLAUDE_CODE_ vars are preserved, since they legitimately locate the
// credentials.
func reviewerEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "CLAUDE_CODE_") || strings.HasPrefix(kv, "CLAUDECODE=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}
