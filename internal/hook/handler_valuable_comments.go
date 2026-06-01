package hook

import "strings"

func init() {
	Register("valuable-comments", asyncReview{
		name:       "valuable-comments",
		fileSuffix: ".go",
		tier:       MediumBalanced, // sonnet by default; override with -tier
		precheck:   containsGoComment,
		rubric:     valuableCommentsRubric,
	}.handler())
}

// containsGoComment reports whether text plausibly introduces a Go comment.
// Deliberately cheap and slightly over-eager: a borderline hit (a `//` inside a
// string, say) just costs one reviewer call that answers PASS, whereas a miss
// would skip review entirely — so we bias toward letting things through.
func containsGoComment(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			return true // whole-line or block comment
		}
		if strings.Contains(line, " //") {
			return true // trailing comment; the leading space skips `://` in URLs
		}
	}
	return false
}

const valuableCommentsRubric = `A comment ADDS VALUE when it tells the reader something the code cannot:

- It explains WHY, not WHAT or HOW — the rationale, trade-off, or a non-obvious choice. ("Retry 3x: upstream 503s for ~1s after a deploy" — not "retry three times".)
- It records a constraint or invariant not enforceable in code: an ordering requirement, a subtle precondition, a known gotcha, or why an obvious-looking simplification is wrong.
- It points at an external source: a link/reference to scheme or schema documentation, an RFC, a ticket, a spec section, or a worked example.
- It clarifies something that genuinely cannot be expressed by a good name or a validation rule.

A comment ADDS NO VALUE when:

- It restates the implementation — narrating what the next line(s) already say ("increment i" above i++; "loop over users" above a range loop).
- It just restates the name of the thing ("// UserID is the user ID", "// Start starts the server", "// constructor for Foo").
- It describes a rule that belongs in, or is already enforced by, a validator — a proto field option (e.g. a buf.validate / protoc-gen-validate constraint) or a Go struct-tag validator annotation ("// must not be empty", "// max 32 chars") — the constraint should live on the field, not in prose.
- It is boilerplate or filler ("// TODO" with no content, banner comments).

Be suspicious of needlessly long comments. A four-plus line comment needs a really good reason to exist — it should be giving genuinely valuable context (a non-obvious rationale, a subtle invariant, a worked example, a doc/spec reference). If a long comment is just verbose restatement, padding, or could be said in a line, flag it to be tightened.

Be conservative. Only flag a comment when it clearly falls in the NO VALUE list. If a comment is borderline, or you lack surrounding context to be sure it is redundant, treat it as fine. A doc comment on an exported identifier (required by lint) should be flagged only if it merely restates the name — never just for existing.`
