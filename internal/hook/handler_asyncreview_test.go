package hook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withAsyncLog points the diagnostic log at a temp file for the duration of a
// test and returns a reader for its contents.
func withAsyncLog(t *testing.T) func() string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "async.log")
	asyncLogOverride = path
	t.Cleanup(func() { asyncLogOverride = "" })
	return func() string {
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return ""
			}
			t.Fatalf("read async log: %v", err)
		}
		return string(b)
	}
}

func asyncReviewWith(review func(prompt, model string) (string, error)) asyncReview {
	return asyncReview{
		name:    "valuable-comments",
		tier:    MediumBalanced,
		rubric:  "rubric",
		summary: "summary:",
		review:  review,
	}
}

// A swallowed reviewer error must still leave a trace — this is the failure
// mode that made a broken (e.g. unauthenticated) reviewer look like a PASS.
func TestAsyncReviewLogsSwallowedError(t *testing.T) {
	readLog := withAsyncLog(t)
	a := asyncReviewWith(func(prompt, model string) (string, error) {
		return "", errors.New("Not logged in")
	})

	out, err := invoke(t, a, "Write", WriteInput{FilePath: "/src/x.go", Content: "x"})
	if out != nil || err != nil {
		t.Fatalf("infra failure must not wake the agent, got out=%+v err=%v", out, err)
	}

	log := readLog()
	if !strings.Contains(log, "\tERROR: Not logged in\t/src/x.go") {
		t.Errorf("log should record the swallowed error, got %q", log)
	}
	if !strings.Contains(log, "\tDISPATCH\t") {
		t.Errorf("log should record the dispatch, got %q", log)
	}
}

func TestAsyncReviewLogsPass(t *testing.T) {
	readLog := withAsyncLog(t)
	a := asyncReviewWith(func(prompt, model string) (string, error) { return "PASS", nil })

	if _, err := invoke(t, a, "Write", WriteInput{FilePath: "/src/x.go", Content: "x"}); err != nil {
		t.Fatalf("PASS should not error, got %v", err)
	}
	if log := readLog(); !strings.Contains(log, "\tPASS\t/src/x.go") {
		t.Errorf("log should record PASS, got %q", log)
	}
}

func TestAsyncReviewLogsRevise(t *testing.T) {
	readLog := withAsyncLog(t)
	a := asyncReviewWith(func(prompt, model string) (string, error) { return "REVISE\n- cut it", nil })

	if _, err := invoke(t, a, "Write", WriteInput{FilePath: "/src/x.go", Content: "x"}); err == nil {
		t.Fatal("REVISE should return an error to wake the agent")
	}
	if log := readLog(); !strings.Contains(log, "\tREVISE\t/src/x.go") {
		t.Errorf("log should record REVISE, got %q", log)
	}
}

// Best-effort logging must never break the hook, even if the path is unwritable.
func TestAsyncReviewLogFailureIsSilent(t *testing.T) {
	asyncLogOverride = filepath.Join(t.TempDir(), "nonexistent-dir", "async.log")
	t.Cleanup(func() { asyncLogOverride = "" })

	a := asyncReviewWith(func(prompt, model string) (string, error) { return "PASS", nil })
	if _, err := invoke(t, a, "Write", WriteInput{FilePath: "/src/x.go", Content: "x"}); err != nil {
		t.Fatalf("unwritable log must not affect the hook, got %v", err)
	}
}
