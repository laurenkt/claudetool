package hook

import "strings"

func init() {
	Register("rpc-wrapper", asyncReview{
		name:       "rpc-wrapper",
		fileSuffix: ".go",
		skipSuffix: "_test.go",
		tier:       MediumBalanced, // sonnet by default; override with -tier
		precheck:   looksLikeRPCWrapper,
		rubric:     rpcWrapperRubric,
		summary:    "Async review flagged a possibly pointless RPC wrapper:",
	}.handler())
}

// looksLikeRPCWrapper gates dispatch to changes that introduce a function
// containing a generated-client RPC call (the `…Request{…}.Send(ctx).DecodeResponse()`
// shape). Over-eager by design: a borderline hit just costs one PASS.
func looksLikeRPCWrapper(text string) bool {
	return strings.Contains(text, "func ") && // a func, not a bare inline call site
		strings.Contains(text, ".Send(") &&
		strings.Contains(text, "DecodeResponse")
}

const rpcWrapperRubric = `You are reviewing a newly written or edited Go function that calls a generated RPC client — the ` + "`<proto>.<Name>Request{…}.Send(ctx).DecodeResponse()`" + ` pattern (or a close variant). Judge ONLY whether the function is a POINTLESS THIN WRAPPER around a single RPC call: one that adds nothing over making the RPC call directly at the call site.

A wrapper is POINTLESS (push back) when its body does essentially only this, and nothing more:

- builds one request, sends it, and decodes the response;
- copies its parameters 1:1 into the request fields with no transformation;
- wraps the error trivially (a single terrors.Augment / fmt.Errorf) or returns it as-is;
- returns the response, or one field plucked off it.

The test: if you deleted this function and inlined the RPC call at each caller, nothing would be lost. That is the smell.

A wrapper ADDS VALUE — do NOT flag — when it does any of:

- orchestrates more than one call, or derives its result by combining several sources;
- validates, defaults, or transforms the inputs beyond a 1:1 field copy;
- maps the generated proto response into a domain type, or otherwise hides the generated types behind a cleaner contract the rest of the package depends on;
- adds behaviour: retry, caching, fallback, rate-limiting, a non-trivial timeout, metrics;
- is a method implementing an interface that exists for dependency injection / mocking (it has a receiver on a client/repository/store-like type).

You cannot see the call sites or whether an interface requires this method. So only flag when the function, on its own, is unmistakably a one-call pass-through with trivial error-wrapping and no other logic. When you do flag it, say to inline the RPC at the call site — and add that if it is in fact an interface seam or shared by many callers, the author should keep it and disregard this note.`
