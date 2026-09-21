package hook

import "strings"

func init() {
	Register("valuable-comments", asyncReview{
		name:         "valuable-comments",
		fileSuffixes: valuableCommentSuffixes,
		tier:         MediumBalanced, // sonnet by default; override with -tier
		precheck:     containsComment,
		rubric:       valuableCommentsRubric,
		summary:      "Async comment review found low-value comments:",
	}.handler())
}

// valuableCommentSuffixes scopes the check to languages the rubric speaks to.
// Shell scripts with no extension are missed — there is no path to match on.
var valuableCommentSuffixes = []string{
	".go",
	".js", ".jsx", ".mjs", ".cjs",
	".ts", ".tsx", ".mts", ".cts",
	".py", ".pyi",
	".sh", ".bash", ".zsh",
	".sql",
}

// containsComment reports whether text plausibly introduces a comment in any
// of the languages this check covers: `//` and `/* */` (Go, JS, TS), `#`
// (Python, shell), `--` (SQL), and `"""` / `”'` (Python docstrings).
//
// Deliberately cheap and slightly over-eager: a borderline hit (a `//` inside
// a string, a JS private field `#count`) just costs one reviewer call that
// answers PASS, whereas a miss would skip review entirely — so we bias toward
// letting things through.
func containsComment(text string) bool {
	// `--` is line-leading only; `cmd --flag` would otherwise fire on most
	// shell edits.
	leading := []string{"//", "/*", "#", "--", `"""`, "'''"}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, marker := range leading {
			if strings.HasPrefix(trimmed, marker) {
				return true
			}
		}
		// Trailing comments; the leading space skips `://` in URLs.
		if strings.Contains(line, " //") || strings.Contains(line, " #") {
			return true
		}
	}
	return false
}

const valuableCommentsRubric = `You are reviewing the comments in a code change. The language is whatever the FILE path below indicates — Go, JavaScript, TypeScript, Python, shell, or SQL. Judge only the comments (including docstrings, JSDoc/TSDoc blocks, and SQL ` + "`--`" + ` notes) — not the code's correctness, style, or naming.

A comment ADDS VALUE when it tells the reader something the code cannot:

- It explains WHY, not WHAT or HOW — the rationale, trade-off, or a non-obvious choice. ("Retry 3x: upstream 503s for ~1s after a deploy" — not "retry three times".)
- It records a constraint or invariant not enforceable in code: an ordering requirement, a subtle precondition, a known gotcha, or why an obvious-looking simplification is wrong.
- It points at an external source: a link/reference to scheme or schema documentation, an RFC, a ticket, a spec section, or a worked example.
- It clarifies something that genuinely cannot be expressed by a good name or a validation rule.

A comment ADDS NO VALUE when:

- It restates the implementation — narrating what the next line(s) already say ("increment i" above i++; "loop over users" above a loop; "select active users" above a WHERE clause that says active).
- It just restates the name of the thing ("// UserID is the user ID", "// Start starts the server", "# constructor for Foo", a docstring reading """Returns the user.""" on get_user).
- It describes a rule that belongs in, or is already enforced by, a validator or the schema — a proto field option (e.g. a buf.validate / protoc-gen-validate constraint), a Go struct-tag validator annotation, a zod/Pydantic schema, or a SQL column constraint ("-- must not be empty", "// max 32 chars") — the constraint should live on the field or column, not in prose.
- It is boilerplate or filler ("TODO" with no content, banner comments, a docstring that only repeats the parameter list and types the signature already declares).
- It refers to the change itself rather than the code's steady state — past edits, the diff, or PR feedback ("now we also do Y", "we need this because we're changing X", "as discussed, switched to Z"). A comment is read in the future, when that change is simply how things have always been; it must justify the code going forward, not narrate how it got here.
- It describes HOW the implementation works. The code already shows how; if the reader can see it by reading the lines below, the comment is dead weight.
- It clarifies something that should be obvious — reassuring the reader that the implementation handles a situation any competent implementation would obviously handle, or won't break in a case where there is no reason to expect it would.
- A doc comment or docstring on a declaration (function, class, type, variable) that explains or justifies what happens INSIDE the body — narrating the branches, listing which cases do what, or arguing why a particular case is handled the way it is. A doc comment documents the contract: what the thing is and how a caller uses it, not how it works inside. If one line of the body genuinely needs justifying, put a few words on that line — never hoist body-level reasoning up into the doc comment.
- It duplicates what a type annotation or signature already states (a JSDoc ` + "`@param {string} name`" + ` on a typed TypeScript parameter, a docstring listing types Python already annotates).
- It trails a field declaration on the same line instead of sitting on its own line above it (` + "`foo string // this is to do a thing`" + `). The comment belongs above the field: trailing notes get squeezed into whatever width is left, drift out of alignment as neighbouring fields change, and push the declaration itself off to the left of the reader's attention. Applies to members of a struct, interface, enum, or const/var block, and to column definitions in SQL DDL. Flag the placement even when the comment is worth keeping — say to move it above the field. (Scoped to declarations in a type, const, or var block; a few words justifying a single statement inside a function body may stay on that line.)
- Notes about individual fields, parameters, or lines are collected into the doc comment above the enclosing function or type, instead of sitting on the thing each one describes. A doc comment that reads as a list of per-field or per-line annotations must be broken up and pushed down — each note goes above its own field, or onto its own statement in the body. Never leave a field's comment hoisted above the func.
- (Go) It sits immediately above the ` + "`package`" + ` clause. That slot is the package doc comment — godoc concatenates it across every file in the package — so a note about what this one file contains ("// Package hook — the hook handlers", "// handler_foo.go implements the foo check", "// This file holds the async review plumbing") is misfiled as documentation for the whole package, and reads as nonsense next to the other files' contributions. Default to flagging it: say to delete it, or move it onto the declaration it actually describes. Only these belong above ` + "`package`" + `: a genuine package doc in the one file that owns it (doc.go, or the package's canonical file), a ` + "`//go:build`" + ` constraint, a ` + "`// Code generated … DO NOT EDIT.`" + ` marker, and a license header.

Default to suspicion of long comments. Two or more lines is already a strong signal the comment is overstaying its welcome — demand a genuinely good reason (a non-obvious rationale, a subtle invariant, a worked example, a doc/spec reference). If a comment is making multiple separate points, it is probably unnecessary: a comment that truly earns its place usually makes one sharp point. Verbose restatement, padding, or anything that could be said in a few words should be flagged to be cut or tightened — strongly prefer no comment over a bloated one.

A doc comment or docstring that a linter requires (a Go exported identifier, a documented public API) should be flagged only if it merely restates the name — never just for existing. A shell shebang, an encoding line, a license header, or a ` + "`# noqa` / `// eslint-disable` / `//nolint`" + ` directive is not a comment for these purposes: ignore it.`
