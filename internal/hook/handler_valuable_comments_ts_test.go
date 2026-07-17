package hook

import (
	"strings"
	"testing"
)

func TestValuableCommentsTSRegistered(t *testing.T) {
	if _, ok := registry["valuable-comments-ts"]; !ok {
		t.Fatal("valuable-comments-ts not registered")
	}
}

func TestValuableCommentsTSRubricIsTypeScript(t *testing.T) {
	a, rec := newReview(t, "PASS")
	a.fileSuffixes = []string{".ts", ".tsx"}
	a.rubric = valuableCommentsRubricTS
	_, _ = invoke(t, a, "Write", WriteInput{
		FilePath: "/src/Panel.tsx",
		Content:  "// renders the sidebar\nfunction Sidebar() {}",
	})
	if !rec.called {
		t.Fatal("reviewer should be called for a .tsx file with a comment")
	}
	if !strings.Contains(rec.prompt, "TypeScript/TSX") ||
		!strings.Contains(rec.prompt, "zod") {
		t.Error("prompt should embed the TypeScript rubric")
	}
}

func TestAsyncReviewMultiSuffixMatching(t *testing.T) {
	a := asyncReview{fileSuffixes: []string{".ts", ".tsx"}}
	cases := []struct {
		path string
		want bool
	}{
		{"/src/x.ts", true},
		{"/src/x.tsx", true},
		{"/src/x.go", false},
		{"/src/x.js", false},
	}
	for _, c := range cases {
		if got := a.matchesSuffix(c.path); got != c.want {
			t.Errorf("matchesSuffix(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestAsyncReviewNoSuffixMatchesEverything(t *testing.T) {
	a := asyncReview{}
	for _, p := range []string{"/src/x.go", "/src/x.tsx", "/README.md"} {
		if !a.matchesSuffix(p) {
			t.Errorf("matchesSuffix(%q) = false, want true (no suffixes configured)", p)
		}
	}
}

func TestAsyncReviewMultiSuffixDispatch(t *testing.T) {
	a, rec := newReview(t, "PASS")
	a.fileSuffixes = []string{".ts", ".tsx"}

	rec.called = false
	_, _ = invoke(t, a, "Write", WriteInput{FilePath: "/src/x.go", Content: "// c\nx()"})
	if rec.called {
		t.Error("reviewer should not run on .go when suffixes are .ts/.tsx")
	}

	rec.called = false
	_, _ = invoke(t, a, "Write", WriteInput{FilePath: "/src/x.tsx", Content: "// c\nx()"})
	if !rec.called {
		t.Error("reviewer should run on .tsx")
	}
}
