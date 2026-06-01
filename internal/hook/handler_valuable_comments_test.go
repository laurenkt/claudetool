package hook

import (
	"encoding/json"
	"strings"
	"testing"
)

// newReview builds an asyncReview wired to a stub reviewer that records the
// prompt and model it was called with.
func newReview(t *testing.T, verdict string) (asyncReview, *struct {
	called bool
	prompt string
	model  string
}) {
	t.Helper()
	rec := &struct {
		called bool
		prompt string
		model  string
	}{}
	a := asyncReview{
		name:       "valuable-comments",
		fileSuffix: ".go",
		tier:       MediumBalanced,
		precheck:   containsGoComment,
		rubric:     valuableCommentsRubric,
		review: func(prompt, model string) (string, error) {
			rec.called = true
			rec.prompt = prompt
			rec.model = model
			return verdict, nil
		},
	}
	return a, rec
}

// invoke runs a handler against a single Write/Edit tool input.
func invoke(t *testing.T, a asyncReview, tool string, toolInput any, args ...string) (*Output, error) {
	t.Helper()
	ti, _ := json.Marshal(toolInput)
	return a.handler()(&Input{
		HookEventName: "PostToolUse",
		ToolName:      tool,
		ToolInput:     ti,
		Args:          args,
	})
}

func TestValuableCommentsRegistered(t *testing.T) {
	if _, ok := registry["valuable-comments"]; !ok {
		t.Fatal("valuable-comments not registered")
	}
}

func TestValuableCommentsPrecheckSkipsNoComment(t *testing.T) {
	a, rec := newReview(t, "PASS")
	out, err := invoke(t, a, "Write", WriteInput{
		FilePath: "/src/x.go",
		Content:  "x := \"https://example.com\"\ni++\n",
	})
	if err != nil || out != nil {
		t.Fatalf("want nil/nil, got out=%+v err=%v", out, err)
	}
	if rec.called {
		t.Error("reviewer should not be called when no comment is present")
	}
}

func TestValuableCommentsSkipsNonGo(t *testing.T) {
	a, rec := newReview(t, "REVISE\n- bad")
	out, err := invoke(t, a, "Write", WriteInput{
		FilePath: "/src/x.py",
		Content:  "# increment\ni += 1\n",
	})
	if err != nil || out != nil {
		t.Fatalf("want nil/nil, got out=%+v err=%v", out, err)
	}
	if rec.called {
		t.Error("reviewer should not be called for non-.go files")
	}
}

func TestValuableCommentsSkipsNonWriteEdit(t *testing.T) {
	a, rec := newReview(t, "REVISE\n- bad")
	out, err := invoke(t, a, "Bash", BashInput{Command: "echo // hi"})
	if err != nil || out != nil {
		t.Fatalf("want nil/nil, got out=%+v err=%v", out, err)
	}
	if rec.called {
		t.Error("reviewer should not be called for non-Write/Edit tools")
	}
}

func TestValuableCommentsPass(t *testing.T) {
	a, rec := newReview(t, "PASS")
	out, err := invoke(t, a, "Edit", EditInput{
		FilePath:  "/src/x.go",
		NewString: "// Retry 3x: upstream 503s for ~1s after a deploy.\nretry(3)",
	})
	if !rec.called {
		t.Fatal("reviewer should be called when a comment is present")
	}
	if err != nil || out != nil {
		t.Fatalf("want nil/nil on PASS, got out=%+v err=%v", out, err)
	}
}

func TestValuableCommentsReviseReturnsError(t *testing.T) {
	a, _ := newReview(t, "REVISE\n- \"// increment i\" — restates i++ -> delete")
	out, err := invoke(t, a, "Edit", EditInput{
		FilePath:  "/src/x.go",
		NewString: "// increment i\ni++",
	})
	if out != nil {
		t.Fatalf("want nil output on REVISE, got %+v", out)
	}
	if err == nil {
		t.Fatal("want error on REVISE (maps to exit 2)")
	}
	if !strings.Contains(err.Error(), "increment i") {
		t.Errorf("error should carry the feedback, got %q", err.Error())
	}
}

func TestValuableCommentsUnknownVerdictPasses(t *testing.T) {
	a, _ := newReview(t, "I think this looks mostly fine honestly")
	out, err := invoke(t, a, "Edit", EditInput{
		FilePath:  "/src/x.go",
		NewString: "// something\nx()",
	})
	if err != nil || out != nil {
		t.Fatalf("unrecognised verdict should be treated as PASS, got out=%+v err=%v", out, err)
	}
}

func TestValuableCommentsPromptCarriesRubricAndCode(t *testing.T) {
	a, rec := newReview(t, "PASS")
	_, _ = invoke(t, a, "Edit", EditInput{
		FilePath:  "/src/x.go",
		NewString: "// loop over users\nfor _, u := range users {}",
	})
	if !strings.Contains(rec.prompt, "ADDS NO VALUE") {
		t.Error("prompt should embed the rubric")
	}
	if !strings.Contains(rec.prompt, "loop over users") {
		t.Error("prompt should embed the changed code")
	}
	if !strings.Contains(rec.prompt, "/src/x.go") {
		t.Error("prompt should name the file")
	}
}

func TestValuableCommentsEditUsesNewStringOnly(t *testing.T) {
	a, rec := newReview(t, "PASS")
	_, _ = invoke(t, a, "Edit", EditInput{
		FilePath:  "/src/x.go",
		OldString: "// stale comment in old code\nx()",
		NewString: "y()",
	})
	if rec.called {
		t.Error("reviewer should see only NewString; OldString comment must not trigger it")
	}
}

func TestReviewTierResolution(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, "sonnet"},                      // default MediumBalanced
		{[]string{"-tier", "fast"}, "haiku"}, // separate value
		{[]string{"-tier=slow"}, "opus"},     // = form
		{[]string{"--tier", "medium"}, "sonnet"},
		{[]string{"-tier", "bogus"}, "sonnet"}, // unknown -> default
	}
	for _, c := range cases {
		a, rec := newReview(t, "PASS")
		_, _ = invoke(t, a, "Edit", EditInput{
			FilePath:  "/src/x.go",
			NewString: "// note\nx()",
		}, c.args...)
		if rec.model != c.want {
			t.Errorf("args %v: model = %q, want %q", c.args, rec.model, c.want)
		}
	}
}
