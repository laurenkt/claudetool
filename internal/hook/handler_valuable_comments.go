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
- It refers to the change itself rather than the code's steady state — past edits, the diff, or PR feedback ("now we also do Y", "we need this because we're changing X", "as discussed, switched to Z"). A comment is read in the future, when that change is simply how things have always been; it must justify the code going forward, not narrate how it got here.
- It describes HOW the implementation works. The code already shows how; if the reader can see it by reading the lines below, the comment is dead weight.
- It clarifies something that should be obvious — reassuring the reader that the implementation handles a situation any competent implementation would obviously handle, or won't break in a case where there is no reason to expect it would.
- A doc comment on a declaration (func, type, var) that explains or justifies what happens INSIDE the body — narrating the branches, listing which cases do what, or arguing why a particular case is handled the way it is. A doc comment documents the contract: what the thing is and how a caller uses it, not how it works inside. If one line of the body genuinely needs justifying, put a few words on that line — never hoist body-level reasoning up into the doc comment.

Default to suspicion of long comments. Two or more lines is already a strong signal the comment is overstaying its welcome — demand a genuinely good reason (a non-obvious rationale, a subtle invariant, a worked example, a doc/spec reference). If a comment is making multiple separate points, it is probably unnecessary: a comment that truly earns its place usually makes one sharp point. Verbose restatement, padding, or anything that could be said in a few words should be flagged to be cut or tightened — strongly prefer no comment over a bloated one.

A doc comment on an exported identifier (required by lint) should be flagged only if it merely restates the name — never just for existing.`
