package hook

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func init() {
	Register("go-augment-style", handleAugmentStyle)
}

var augmentBadPrefix = regexp.MustCompile(`terrors\.Augment\([^"]*"(failed to |error |unable to |could not |couldn't )`)

// handleAugmentStyle blocks Write/Edit calls that introduce terrors.Augment
// context strings with verbose prefixes like "failed to read payment" instead
// of concise action descriptions like "read payment".
//
// Use as a PostToolUse hook with matcher "Write|Edit".
func handleAugmentStyle(in *Input) (*Output, error) {
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

	matches := augmentBadPrefix.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil, nil
	}

	return &Output{
		Decision: "block",
		Reason: fmt.Sprintf(`terrors.Augment context strings get concatenated in error chains like "check account open: read account: lock: context cancelled". Use concise action descriptions, not verbose prefixes.

Bad:  terrors.Augment(err, "failed to read account", ...)
Good: terrors.Augment(err, "read account", ...)

Found %d match(es):
  %s`, len(matches), strings.Join(matches, "\n  ")),
	}, nil
}
