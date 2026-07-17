package hook

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

func init() {
	Register("react-named-hooks", handleReactNamedHooks)
}

// reReactHook matches a `React.useXxx(` hook call. Requiring `use`+uppercase
// skips non-hook members (React.FC, React.ReactNode); requiring `(` skips type
// positions, since a hook is always called.
var reReactHook = regexp.MustCompile(`\bReact\.(use[A-Z]\w*)\s*\(`)

// handleReactNamedHooks flags hooks called off the React namespace
// (`React.useState(...)`) and suggests importing them by name. Advisory,
// non-blocking. Use as a PostToolUse hook with matcher "Write|Edit".
func handleReactNamedHooks(in *Input) (*Output, error) {
	if in.ToolName != "Write" && in.ToolName != "Edit" {
		return nil, nil
	}

	filePath, text := changedContent(in)
	if filePath == "" {
		return nil, nil
	}
	if !hasSuffixAny(filePath, ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs") {
		return nil, nil
	}

	seen := map[string]bool{}
	var hits []string
	for i, line := range strings.Split(text, "\n") {
		for _, m := range reReactHook.FindAllStringSubmatch(line, -1) {
			seen[m[1]] = true
			hits = append(hits, fmt.Sprintf("  line %d: %s", i+1, strings.TrimSpace(line)))
		}
	}
	if len(hits) == 0 {
		return nil, nil
	}

	var names []string
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)

	msg := fmt.Sprintf("Style suggestion (advisory, not blocking): import React hooks by name instead of calling them off the `React` namespace.\n\n"+
		"Bad:  const [x, setX] = React.useState(0)\n"+
		"Good: import { useState } from 'react'\n"+
		"      const [x, setX] = useState(0)\n\n"+
		"Found %d call(s) off `React.` (%s) in %s:\n%s",
		len(hits), strings.Join(names, ", "), filePath, strings.Join(hits, "\n"))

	return &Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:     in.HookEventName,
			AdditionalContext: msg,
		},
	}, nil
}

func hasSuffixAny(s string, suffixes ...string) bool {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}
