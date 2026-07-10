package hook

import (
	"fmt"
	"regexp"
	"strings"
)

func init() {
	Register("go-cmp-or", handleCmpOr)
}

// reNonEmptyGuard matches `if X != "" {`, capturing X.
var reNonEmptyGuard = regexp.MustCompile(`^\s*if\s+([\w.]+)\s*!=\s*""\s*{`)

// reEmptyReturn matches the bare `return ""` fallback.
var reEmptyReturn = regexp.MustCompile(`^\s*return\s+""\s*$`)

// handleCmpOr flags hand-rolled first-non-empty-string selectors — a value
// returned under an `x != ""` guard alongside a `return ""` fallback, whether
// written as a range loop or a chain of ifs — and points at cmp.Or.
//
// Use as a PostToolUse hook with matcher "Write|Edit".
func handleCmpOr(in *Input) (*Output, error) {
	filePath, text := changedContent(in)
	if !strings.HasSuffix(filePath, ".go") {
		return nil, nil
	}

	lines := strings.Split(text, "\n")

	hasEmptyFallback := false
	for _, line := range lines {
		if reEmptyReturn.MatchString(line) {
			hasEmptyFallback = true
			break
		}
	}
	if !hasEmptyFallback {
		return nil, nil
	}

	var hits []string
	for i, line := range lines {
		m := reNonEmptyGuard.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if returnsIdent(lines, i, m[1]) {
			hits = append(hits, fmt.Sprintf("  line %d: %s", i+1, strings.TrimSpace(line)))
		}
	}
	if len(hits) == 0 {
		return nil, nil
	}

	msg := fmt.Sprintf(`Style suggestion (advisory, not blocking): this looks like a hand-rolled "first non-empty" selector. Use `+"`cmp.Or`"+` (Go 1.22+) instead — it returns the first non-zero argument, or the zero value if all are zero, and works for any comparable type.

Bad:  func firstNonEmpty(values ...string) string {
          for _, v := range values {
              if v != "" {
                  return v
              }
          }
          return ""
      }
      name := firstNonEmpty(a, b, c)
Good: name := cmp.Or(a, b, c)

Found %d match(es) in %s:
%s`, len(hits), filePath, strings.Join(hits, "\n"))

	return &Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:     in.HookEventName,
			AdditionalContext: msg,
		},
	}, nil
}

// returnsIdent reports whether the `if X != "" {` at lines[i] returns X.
func returnsIdent(lines []string, i int, ident string) bool {
	retRe := regexp.MustCompile(`\breturn\s+` + regexp.QuoteMeta(ident) + `\b`)
	if retRe.MatchString(lines[i]) {
		return true
	}
	for j := i + 1; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "" {
			continue
		}
		return retRe.MatchString(lines[j])
	}
	return false
}
