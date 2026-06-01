package hook

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func init() {
	Register("go-makeslice", handleMakeSlice)
}

// reMakeSliceZeroLen matches `make([]T, 0, X)` — a zero-length slice with a
// capacity hint, typically used right before a series of append() calls.
// `var x []T` reads more cleanly; the capacity hint rarely matters.
var reMakeSliceZeroLen = regexp.MustCompile(`make\(\s*\[\][^,)]+,\s*0\s*,`)

// handleMakeSlice flags `make([]T, 0, X)` patterns as a non-blocking
// stylistic suggestion: prefer `var x []T` for clarity.
//
// Use as a PostToolUse hook with matcher "Write|Edit".
func handleMakeSlice(in *Input) (*Output, error) {
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

	var hits []string
	for i, line := range strings.Split(text, "\n") {
		if reMakeSliceZeroLen.MatchString(line) {
			hits = append(hits, fmt.Sprintf("  line %d: %s", i+1, strings.TrimSpace(line)))
		}
	}
	if len(hits) == 0 {
		return nil, nil
	}

	msg := fmt.Sprintf(`Style suggestion (advisory, not blocking): prefer `+"`var x []T`"+` over `+"`make([]T, 0, n)`"+` — it reads with less noise, and the capacity hint rarely matters.

Bad:  out := make([]Event, 0, len(events))
      for _, e := range events {
          out = append(out, convert(e))
      }
Good: var out []Event
      for _, e := range events {
          out = append(out, convert(e))
      }

Found %d match(es) in %s:
%s`, len(hits), filePath, strings.Join(hits, "\n"))

	return &Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:     in.HookEventName,
			AdditionalContext: msg,
		},
	}, nil
}
