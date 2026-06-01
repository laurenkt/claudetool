package hook

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

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
	name       string                 // handler name, e.g. "valuable-comments"
	fileSuffix string                 // required file suffix, e.g. ".go"
	skipSuffix string                 // optional suffix to skip, e.g. "_test.go" ("" = none)
	tier       ReviewTier             // default model tier
	precheck   func(text string) bool // cheap gate: only dispatch when true (nil = always)
	rubric     string                 // review criteria embedded in the reviewer prompt

	// review runs the reviewer and returns its raw output. Injected in tests;
	// nil falls back to runClaudeReview against the real `claude` binary.
	review func(prompt, model string) (string, error)
}

// handler returns a Handler closure for use with Register.
func (a asyncReview) handler() Handler {
	return func(in *Input) (*Output, error) {
		if in.ToolName != "Write" && in.ToolName != "Edit" {
			return nil, nil
		}

		filePath, text := changedContent(in)
		if filePath == "" {
			return nil, nil
		}
		if a.fileSuffix != "" && !strings.HasSuffix(filePath, a.fileSuffix) {
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

		out, err := review(buildReviewPrompt(a.rubric, filePath, text), tier.model())
		if err != nil {
			// Reviewer unavailable (claude not on PATH, timeout, etc.).
			// Best-effort: never wake the agent on infrastructure failure.
			return nil, nil
		}

		verdict, feedback := parseVerdict(out)
		if verdict != "REVISE" {
			return nil, nil
		}

		// exit 2 with the feedback on stderr; under asyncRewake this is what
		// surfaces back to the working agent.
		return nil, fmt.Errorf("%s", feedback)
	}
}

// changedContent extracts the file path and the newly written/edited text from
// a Write or Edit tool input.
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
			body = "The reviewer flagged a comment but gave no detail."
		}
		return "REVISE", "Async comment review found low-value comments:\n\n" + body
	default:
		return "PASS", ""
	}
}

// reviewerSystemPrompt frames the headless reviewer.
const reviewerSystemPrompt = `You are a code-comment reviewer. You are given a rubric and a snippet of newly written or edited code. Judge ONLY the comments in the snippet against the rubric — not the code's correctness, style, or naming. Follow the output protocol exactly: no preamble, no markdown fences, nothing else.`

// buildReviewPrompt assembles the prompt piped to `claude -p` on stdin.
func buildReviewPrompt(rubric, filePath, text string) string {
	return fmt.Sprintf(`%s

OUTPUT PROTOCOL
If every comment in the changed code adds value (or there are no comments), respond with exactly:
PASS

Otherwise respond with:
REVISE
- "<the weak comment>" — <why it adds no value> -> <delete | rewrite as why | move to a validator>
(one bullet per weak comment, nothing after the list)

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
//   - `--disallowed-tools` keeps the reviewer to pure text analysis; it never
//     needs to touch the filesystem or run commands.
func runClaudeReview(prompt, model string) (string, error) {
	cmd := exec.Command("claude",
		"-p",
		"--model", model,
		"--settings", "{}",
		"--disallowed-tools", "Bash Edit Write Read Glob Grep WebFetch WebSearch",
		"--append-system-prompt", reviewerSystemPrompt,
	)
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
