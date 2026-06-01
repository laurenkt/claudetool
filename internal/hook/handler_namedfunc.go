package hook

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func init() {
	Register("go-named-func", handleNamedFunc)
}

// reNamedAnonFunc matches a statement-level assignment of a function literal
// to a named variable: `name := func(...)` or `name = func(...)` or
// `var name = func(...)`. The line-start anchor keeps the rule away from
// function literals passed as call arguments.
var reNamedAnonFunc = regexp.MustCompile(`^\s*(?:var\s+)?\w+\s*(?::=|=)\s*func\s*\(`)

// handleNamedFunc flags named anonymous functions — `addCopyable := func(...) { ... }`
// — as a non-blocking stylistic suggestion: if it deserves a name, hoist it
// to a real function declaration.
//
// Skipped on `_test.go` (table-driven helpers often warrant this shape).
// Use as a PostToolUse hook with matcher "Write|Edit".
func handleNamedFunc(in *Input) (*Output, error) {
	if in.ToolName != "Write" && in.ToolName != "Edit" {
		return nil, nil
	}

	var filePath, text string

	switch in.ToolName {
	case "Write":
		var w WriteInput
		if err := json.Unmarshal(in.ToolInput, &w); err != nil {
			return nil, nil
		}
		filePath = w.FilePath
		text = w.Content
	case "Edit":
		var e EditInput
		if err := json.Unmarshal(in.ToolInput, &e); err != nil {
			return nil, nil
		}
		filePath = e.FilePath
		text = e.NewString
	}

	if !strings.HasSuffix(filePath, ".go") {
		return nil, nil
	}
	if strings.HasSuffix(filePath, "_test.go") {
		return nil, nil
	}

	var hits []string
	for i, line := range strings.Split(text, "\n") {
		if reNamedAnonFunc.MatchString(line) {
			hits = append(hits, fmt.Sprintf("  line %d: %s", i+1, strings.TrimSpace(line)))
		}
	}
	if len(hits) == 0 {
		return nil, nil
	}

	msg := fmt.Sprintf(`Style suggestion (advisory, not blocking): avoid named anonymous functions. They're almost always a sign of being too clever.

The default fix is NOT to hoist them into a real `+"`func`"+` declaration — that's also usually overkill. Just write the code straight, and don't be afraid of a little repetition. A few near-identical lines in a row read more clearly than an indirection that the reader has to mentally evaluate.

Only keep a named anonymous function if there's a real need: e.g. it closes over local state in a way that genuinely can't be expressed without a closure, and extracting it would require threading several variables through a parameter list.

Anonymous functions passed as arguments (sort comparators, filter predicates, etc.) are fine and are not flagged.

Found %d match(es) in %s:
%s`, len(hits), filePath, strings.Join(hits, "\n"))

	return &Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:     in.HookEventName,
			AdditionalContext: msg,
		},
	}, nil
}
