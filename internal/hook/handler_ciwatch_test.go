package hook

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/laurenkt/claudetool/internal/ghutil"
)

// noopPreempt stands in for the real pidfile/process-group plumbing so tests
// never call syscall.Setpgid on the test process.
func noopPreempt(repoKey, branch string) (func(), error) {
	return func() {}, nil
}

// cwd must be a real git repo dir for gitutil.Branch to resolve a branch; the
// test package dir (inside this repo) works, resolved to an absolute path so
// the .git walk-up can escape it.
func ciInvoke(t *testing.T, c ciWatch, command, cwd string) (*Output, error) {
	t.Helper()
	abs, err := filepath.Abs(cwd)
	if err != nil {
		t.Fatalf("abs(%q): %v", cwd, err)
	}
	c.preempt = noopPreempt
	ti, _ := json.Marshal(BashInput{Command: command})
	return c.handler()(&Input{
		HookEventName: "PostToolUse",
		ToolName:      "Bash",
		ToolInput:     ti,
		CWD:           abs,
	})
}

func openPR(failing ...ghutil.Check) func(context.Context, string, string) (ghutil.PRInfo, bool) {
	return func(context.Context, string, string) (ghutil.PRInfo, bool) {
		r := ghutil.Rollup{Total: 2, Failed: len(failing), Failing: failing}
		return ghutil.PRInfo{Number: 99, HeadSHA: "abcdef1234567", Rollup: r}, true
	}
}

func TestCIWatchRegistered(t *testing.T) {
	if _, ok := registry["ci-watch"]; !ok {
		t.Fatal("ci-watch not registered")
	}
}

func TestCIWatchIgnoresNonBash(t *testing.T) {
	c := ciWatch{
		prForBranch: openPR(),
		watchChecks: func(context.Context, string, string) error { t.Fatal("should not watch"); return nil },
	}
	ti, _ := json.Marshal(WriteInput{FilePath: "/x.go", Content: "git push"})
	out, err := c.handler()(&Input{ToolName: "Write", ToolInput: ti})
	if out != nil || err != nil {
		t.Errorf("want nil/nil for Write, got out=%v err=%v", out, err)
	}
}

func TestCIWatchIgnoresNonPushCommand(t *testing.T) {
	c := ciWatch{
		prForBranch: func(context.Context, string, string) (ghutil.PRInfo, bool) {
			t.Fatal("should not query PR for a non-push command")
			return ghutil.PRInfo{}, false
		},
	}
	out, err := ciInvoke(t, c, "git status", ".")
	if out != nil || err != nil {
		t.Errorf("want nil/nil, got out=%v err=%v", out, err)
	}
}

func TestCIWatchNoPR(t *testing.T) {
	watched := false
	c := ciWatch{
		prForBranch: func(context.Context, string, string) (ghutil.PRInfo, bool) {
			return ghutil.PRInfo{}, false
		},
		watchChecks: func(context.Context, string, string) error { watched = true; return nil },
	}
	out, err := ciInvoke(t, c, "git push origin HEAD", ".")
	if out != nil || err != nil {
		t.Errorf("want nil/nil when no PR, got out=%v err=%v", out, err)
	}
	if watched {
		t.Error("watchChecks called despite no PR")
	}
}

func TestCIWatchAllPass(t *testing.T) {
	c := ciWatch{
		prForBranch: openPR(),
		watchChecks: func(context.Context, string, string) error { return nil },
		failingChecks: func(context.Context, string, string) []ghutil.Check {
			t.Fatal("failingChecks should not be called on all-pass")
			return nil
		},
	}
	out, err := ciInvoke(t, c, "git push", ".")
	if out != nil || err != nil {
		t.Errorf("want nil/nil on all-pass, got out=%v err=%v", out, err)
	}
}

func TestCIWatchFailure(t *testing.T) {
	c := ciWatch{
		prForBranch: openPR(ghutil.Check{Name: "unit-tests", URL: "https://ci/unit"}),
		watchChecks: func(context.Context, string, string) error { return errors.New("exit status 1") },
		failingChecks: func(context.Context, string, string) []ghutil.Check {
			return []ghutil.Check{{Name: "unit-tests", URL: "https://ci/unit"}}
		},
	}
	out, err := ciInvoke(t, c, "gh pr create --fill", ".")
	if out != nil {
		t.Errorf("want nil output, got %v", out)
	}
	if err == nil {
		t.Fatal("want error (exit 2), got nil")
	}
	msg := err.Error()
	for _, want := range []string{"PR #99", "abcdef1", "unit-tests", "https://ci/unit"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
}

func TestCIWatchFailureWithoutNamedChecks(t *testing.T) {
	// watchChecks reported failure but the rollup shows none (e.g. gh errored):
	// stay silent rather than wake the agent on an unconfirmed failure.
	c := ciWatch{
		prForBranch:   openPR(),
		watchChecks:   func(context.Context, string, string) error { return errors.New("gh: command not found") },
		failingChecks: func(context.Context, string, string) []ghutil.Check { return nil },
	}
	out, err := ciInvoke(t, c, "git push", ".")
	if out != nil || err != nil {
		t.Errorf("want nil/nil when no failing checks confirmed, got out=%v err=%v", out, err)
	}
}

func TestCIWatchTimeoutSilent(t *testing.T) {
	c := ciWatch{
		prForBranch: openPR(ghutil.Check{Name: "build"}),
		watchChecks: func(context.Context, string, string) error { return context.DeadlineExceeded },
		failingChecks: func(context.Context, string, string) []ghutil.Check {
			t.Fatal("failingChecks should not be called on timeout")
			return nil
		},
	}
	out, err := ciInvoke(t, c, "gh pr ready", ".")
	if out != nil || err != nil {
		t.Errorf("want nil/nil on timeout, got out=%v err=%v", out, err)
	}
}

func TestIsPushCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"git push", true},
		{"git push origin HEAD", true},
		{"git   push --force-with-lease", true},
		{"gh pr create --fill", true},
		{"gh pr ready 123", true},
		{"cd foo && git push", true},
		{"git status && gh pr create", true},
		{"git push --dry-run", false},
		{"git push --help", false},
		{"gh pr create --help", false},
		{"git status", false},
		{"gh pr view", false},
		{"gh pr list", false},
		{"echo git push", true}, // word-boundary still matches; acceptable over-trigger
		{"mygit push", false},
		{"git pushover", false},
	}
	for _, tt := range tests {
		if got := isPushCommand(tt.cmd); got != tt.want {
			t.Errorf("isPushCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}
