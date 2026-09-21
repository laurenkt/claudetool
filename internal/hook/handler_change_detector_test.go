package hook

import (
	"strings"
	"testing"
)

func newDetectorReview(t *testing.T, verdict string) (asyncReview, *bool) {
	t.Helper()
	called := new(bool)
	a := asyncReview{
		name:         "change-detector-tests",
		fileSuffixes: []string{"_test.go"},
		tier:         MediumBalanced,
		precheck:     looksLikeTestLogic,
		rubric:       changeDetectorRubric,
		summary:      "Async test review flagged a possible change-detector test:",
		review: func(prompt, model string) (string, error) {
			*called = true
			return verdict, nil
		},
	}
	return a, called
}

func TestChangeDetectorRegistered(t *testing.T) {
	if _, ok := registry["change-detector-tests"]; !ok {
		t.Fatal("change-detector-tests not registered")
	}
}

func TestChangeDetectorOnlyTestFiles(t *testing.T) {
	a, called := newDetectorReview(t, "REVISE\n- mirrors impl")
	out, err := invoke(t, a, "Edit", EditInput{
		FilePath:  "/src/x.go", // not a _test.go file
		NewString: "want := compute(); if got != want { t.Errorf(\"\") }",
	})
	if out != nil || err != nil {
		t.Fatalf("want nil/nil for non-test file, got out=%+v err=%v", out, err)
	}
	if *called {
		t.Error("reviewer should not run on non-_test.go files")
	}
}

func TestChangeDetectorPrecheckSkipsNonTestLogic(t *testing.T) {
	a, called := newDetectorReview(t, "REVISE\n- x")
	out, err := invoke(t, a, "Edit", EditInput{
		FilePath:  "/src/x_test.go",
		NewString: "import \"strings\"\n\nvar fixture = strings.Repeat(\"a\", 3)",
	})
	if out != nil || err != nil {
		t.Fatalf("want nil/nil, got out=%+v err=%v", out, err)
	}
	if *called {
		t.Error("reviewer should not run when the change has no test logic")
	}
}

func TestChangeDetectorPass(t *testing.T) {
	a, called := newDetectorReview(t, "PASS")
	out, err := invoke(t, a, "Edit", EditInput{
		FilePath:  "/src/x_test.go",
		NewString: "func TestRoundTrip(t *testing.T) { want := \"x\"; if got := parse(enc(want)); got != want { t.Errorf(\"\") } }",
	})
	if !*called {
		t.Fatal("reviewer should run on test-logic changes")
	}
	if out != nil || err != nil {
		t.Fatalf("want nil/nil on PASS, got out=%+v err=%v", out, err)
	}
}

func TestChangeDetectorReviseReturnsError(t *testing.T) {
	a, _ := newDetectorReview(t, "REVISE\n- asserts the exact mock call sequence -> assert on the returned value instead")
	out, err := invoke(t, a, "Edit", EditInput{
		FilePath:  "/src/x_test.go",
		NewString: "func TestThing(t *testing.T) { mock.AssertCalled(t, \"Save\"); mock.AssertCalled(t, \"Commit\") }",
	})
	if out != nil {
		t.Fatalf("want nil output on REVISE, got %+v", out)
	}
	if err == nil || !strings.Contains(err.Error(), "mock call sequence") {
		t.Fatalf("want error carrying the feedback, got %v", err)
	}
	if !strings.Contains(err.Error(), "change-detector test") {
		t.Errorf("error should carry the summary header, got %q", err.Error())
	}
}
