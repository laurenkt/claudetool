package hook

import "strings"

func init() {
	Register("change-detector-tests", asyncReview{
		name:         "change-detector-tests",
		fileSuffixes: []string{"_test.go"},
		tier:         MediumBalanced, // sonnet by default; override with -tier
		precheck:     looksLikeTestLogic,
		rubric:       changeDetectorRubric,
		summary:      "Async test review flagged a possible change-detector test:",
	}.handler())
}

// looksLikeTestLogic gates dispatch to edits that actually touch assertions or
// test functions, so we don't spawn a reviewer for import shuffles, helper
// tweaks, or formatting. Over-eager by design: a borderline hit just costs one
// PASS.
func looksLikeTestLogic(text string) bool {
	needles := []string{
		"func Test", "func Benchmark", "func Fuzz",
		"assert", "require.", "cmp.Diff", "cmp.Equal",
		"reflect.DeepEqual", ".Equal(",
		"want", "expected", "golden", "snapshot",
		"t.Error", "t.Fatal", "Errorf", "Fatalf",
	}
	for _, n := range needles {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}

const changeDetectorRubric = `You are reviewing a change to a Go _test.go file. Judge whether the added or edited test is essentially a CHANGE-DETECTOR TEST: one coupled to the implementation rather than to observable behaviour, so it breaks whenever the code changes even though nothing is actually broken, and carries little or no defect-detection value.

Signs a test IS a pointless change detector (push back):

- It restates the implementation — asserts a constant, literal, or formula that simply mirrors what the production code computes.
- Over-mocking: it mocks collaborators and then asserts the exact sequence or number of internal calls, so the test mirrors the code's internal call graph rather than any result a caller observes.
- It asserts on private or intermediate internal state rather than the behaviour or output a real caller depends on.
- A snapshot/golden assertion over an arbitrary serialized blob that exists only to notice "did the output change", with no sense that the shape is a contract — the kind that gets blindly re-baselined.
- Updating it after a code change would be a purely mechanical mirror of that change, and it could not catch a realistic bug.

Do NOT flag these — they "detect change" on purpose, and detecting the change is the point:

- Golden/approval tests for code generators, compilers, formatters, or other codegen where the emitted output IS the contract and the diff is meant to be reviewed.
- Message/format parsing, serialization round-trips, wire-format or protobuf field-compatibility tests — a change there breaks clients or persisted data, so failing loudly is correct.
- Characterization / golden-master tests that deliberately pin current behaviour as a safety net while refactoring legacy code.
- Security or config tripwires: tests asserting an allowlist, the set of public endpoints, permissions, or registered handlers, where a silent change should force human review.
- Any test bound to an externally-observable contract or invariant that genuinely should not change without someone noticing.

The discriminator: does the assertion track an externally-observable contract or behaviour (fine), or just the current shape of the implementation (the smell)? Only flag when a test clearly tracks implementation shape and would catch no real bug. Say what behaviour it should assert instead, or that it should be removed.`
