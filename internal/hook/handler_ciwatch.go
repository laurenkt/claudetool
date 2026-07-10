package hook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/laurenkt/claudetool/internal/ghutil"
	"github.com/laurenkt/claudetool/internal/gitutil"
)

func init() {
	Register("ci-watch", ciWatch{}.handler())
}

const (
	ciWatchTimeout      = 30 * time.Minute // give up silently after this
	ciWatchQueryTimeout = 10 * time.Second // per gh pr view query
	ciWatchPollEvery    = 10 * time.Second // gap between race-guard polls
	ciWatchPollRetries  = 6                // ~1 min waiting for checks to register
)

// ciWatch watches a PR's CI checks in the background after a push-ish command
// and wakes the agent (exit 2) only when a check fails. It is meant to run as
// its own PostToolUse (Bash) hook with `asyncRewake: true`: the process blocks
// on `gh pr checks --watch` for up to 30 minutes without holding up the agent,
// and stays silent on success or timeout.
//
// Fields are injectable seams (nil → real implementation) for testing.
type ciWatch struct {
	prForBranch   func(ctx context.Context, dir, branch string) (ghutil.PRInfo, bool)
	watchChecks   func(ctx context.Context, dir, branch string) error
	failingChecks func(ctx context.Context, dir, branch string) []ghutil.Check
	preempt       func(repoKey, branch string) (func(), error)

	timeout     time.Duration // 0 → ciWatchTimeout
	pollEvery   time.Duration // 0 → ciWatchPollEvery
	pollRetries int           // 0 → ciWatchPollRetries
}

func (c ciWatch) handler() Handler {
	prForBranch := c.prForBranch
	if prForBranch == nil {
		prForBranch = ghutil.PRForBranch
	}
	watchChecks := c.watchChecks
	if watchChecks == nil {
		watchChecks = ghutil.WatchChecks
	}
	failingChecks := c.failingChecks
	if failingChecks == nil {
		failingChecks = ghutil.FailingChecks
	}
	preempt := c.preempt
	if preempt == nil {
		preempt = registerWatcher
	}
	timeout := c.timeout
	if timeout == 0 {
		timeout = ciWatchTimeout
	}
	pollEvery := c.pollEvery
	if pollEvery == 0 {
		pollEvery = ciWatchPollEvery
	}
	pollRetries := c.pollRetries
	if pollRetries == 0 {
		pollRetries = ciWatchPollRetries
	}

	return func(in *Input) (*Output, error) {
		if in.ToolName != "Bash" {
			return nil, nil
		}
		var bash BashInput
		if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
			return nil, nil
		}
		if !isPushCommand(bash.Command) {
			return nil, nil
		}
		branch := gitutil.Branch(in.CWD)
		if branch == "" {
			return nil, nil
		}

		// No PR yet (a push before `gh pr create`) means nothing to watch.
		pr, ok := queryPR(prForBranch, in.CWD, branch)
		if !ok {
			return nil, nil
		}

		// A newer push for this branch preempts us; we register so the next one
		// can preempt in turn, and clean up the pidfile on exit.
		cleanup, err := preempt(repoKey(in.CWD), branch)
		if err != nil {
			return nil, nil // best-effort; never wake the agent on plumbing errors
		}
		defer cleanup()

		// Race guard: CI often hasn't registered any checks in the instant after
		// a push. Poll briefly so "no checks yet" is never mistaken for "passed".
		for attempt := 0; pr.Rollup.Total == 0 && attempt < pollRetries; attempt++ {
			time.Sleep(pollEvery)
			pr, ok = queryPR(prForBranch, in.CWD, branch)
			if !ok {
				return nil, nil
			}
		}
		if pr.Rollup.Total == 0 {
			return nil, nil // no checks ever appeared; never report a failure
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		switch err := watchChecks(ctx, in.CWD, branch); {
		case err == nil:
			return nil, nil
		case errors.Is(err, context.DeadlineExceeded):
			return nil, nil
		}

		fctx, fcancel := context.WithTimeout(context.Background(), ciWatchQueryTimeout)
		failing := failingChecks(fctx, in.CWD, branch)
		fcancel()
		if len(failing) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("CI failed on PR #%d (commit %s):\n%s", pr.Number, shortSHA(pr.HeadSHA), formatChecks(failing))
	}
}

func queryPR(fn func(context.Context, string, string) (ghutil.PRInfo, bool), dir, branch string) (ghutil.PRInfo, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), ciWatchQueryTimeout)
	defer cancel()
	return fn(ctx, dir, branch)
}

// Push-ish commands worth watching: a branch push, opening a PR, or marking one
// ready. Word-boundary anchored ((^|[^\w-]) avoids matching inside longer
// tokens) and lower-cased before matching.
var (
	gitPushRe  = regexp.MustCompile(`(^|[^\w-])git\s+push\b`)
	ghCreateRe = regexp.MustCompile(`(^|[^\w-])gh\s+pr\s+create\b`)
	ghReadyRe  = regexp.MustCompile(`(^|[^\w-])gh\s+pr\s+ready\b`)
)

func isPushCommand(cmd string) bool {
	lower := strings.ToLower(cmd)
	if strings.Contains(lower, "--dry-run") || strings.Contains(lower, "--help") {
		return false
	}
	return gitPushRe.MatchString(lower) || ghCreateRe.MatchString(lower) || ghReadyRe.MatchString(lower)
}

func formatChecks(checks []ghutil.Check) string {
	var lines []string
	for _, c := range checks {
		if c.URL != "" {
			lines = append(lines, fmt.Sprintf("- %s (%s)", c.Name, c.URL))
		} else {
			lines = append(lines, "- "+c.Name)
		}
	}
	return strings.Join(lines, "\n")
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// repoKey turns a repo directory into a filesystem-safe pidfile-name component.
func repoKey(dir string) string {
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return sanitizeKey(abs)
}

func sanitizeKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// registerWatcher preempts any live watcher for this repo+branch and records
// this process as the new one. It puts us in our own process group so that the
// preempt signal (sent to the group) also kills the blocking `gh pr checks`
// child. The returned cleanup removes the pidfile if it still points at us.
func registerWatcher(repoKey, branch string) (func(), error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".claude", "ci-watch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	pidPath := filepath.Join(dir, repoKey+"-"+sanitizeKey(branch)+".pid")
	self := os.Getpid()

	if data, err := os.ReadFile(pidPath); err == nil {
		if old, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && old > 0 && old != self {
			_ = syscall.Kill(-old, syscall.SIGTERM) // kill the old watcher's group
		}
	}
	_ = syscall.Setpgid(0, 0)

	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(self)), 0o644); err != nil {
		return nil, err
	}

	return func() {
		if data, err := os.ReadFile(pidPath); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid != self {
				return // a newer watcher owns the pidfile now
			}
		}
		_ = os.Remove(pidPath)
	}, nil
}
