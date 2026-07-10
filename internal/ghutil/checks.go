package ghutil

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
)

// runGH executes a gh subcommand in dir and returns its stdout. Exposed as a
// package var so tests can stub the subprocess without a real `gh` binary.
var runGH = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = dir
	return cmd.Output()
}

// Check is a single CI check on a PR, with a link to its details.
type Check struct {
	Name string
	URL  string
}

// Rollup summarises a PR's statusCheckRollup.
type Rollup struct {
	Total   int
	Pending int
	Failed  int
	Failing []Check
}

// PRInfo carries a PR's identity and the state of its checks.
type PRInfo struct {
	Number  int
	HeadSHA string
	Rollup  Rollup
}

// PRForBranch looks up the open PR for branch and returns its number, head SHA,
// and check rollup. ok is false when there is no PR (the "pushed before a PR
// exists" case) or on any gh error — callers treat that as "nothing to watch".
func PRForBranch(ctx context.Context, dir, branch string) (PRInfo, bool) {
	out, err := runGH(ctx, dir, "pr", "view", branch, "--json", "number,state,headRefOid,statusCheckRollup")
	if err != nil {
		return PRInfo{}, false
	}
	return parsePRView(out)
}

// WatchChecks blocks on `gh pr checks --watch --fail-fast`, returning nil once
// every check has passed and a non-nil error on the first failure. ctx carries
// the watch deadline; on timeout the returned error wraps ctx.Err() so callers
// can tell a give-up from a genuine failure.
func WatchChecks(ctx context.Context, dir, branch string) error {
	_, err := runGH(ctx, dir, "pr", "checks", branch, "--watch", "--fail-fast")
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// FailingChecks returns the checks on branch's PR that have failed.
func FailingChecks(ctx context.Context, dir, branch string) []Check {
	pr, ok := PRForBranch(ctx, dir, branch)
	if !ok {
		return nil
	}
	return pr.Rollup.Failing
}

// rollupEntry covers both shapes gh emits in statusCheckRollup: CheckRun (GitHub
// Actions / check API, keyed on name/status/conclusion) and StatusContext
// (legacy commit statuses, keyed on context/state).
type rollupEntry struct {
	Typename   string `json:"__typename"`
	Name       string `json:"name"`
	Context    string `json:"context"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
	DetailsURL string `json:"detailsUrl"`
	TargetURL  string `json:"targetUrl"`
}

func parsePRView(out []byte) (PRInfo, bool) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return PRInfo{}, false
	}
	var raw struct {
		Number            int           `json:"number"`
		State             string        `json:"state"`
		HeadRefOid        string        `json:"headRefOid"`
		StatusCheckRollup []rollupEntry `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return PRInfo{}, false
	}
	if raw.Number == 0 {
		return PRInfo{}, false
	}
	return PRInfo{
		Number:  raw.Number,
		HeadSHA: raw.HeadRefOid,
		Rollup:  classifyRollup(raw.StatusCheckRollup),
	}, true
}

func classifyRollup(entries []rollupEntry) Rollup {
	var r Rollup
	for _, e := range entries {
		r.Total++
		switch outcome(e) {
		case outcomePending:
			r.Pending++
		case outcomeFailed:
			r.Failed++
			r.Failing = append(r.Failing, Check{Name: checkName(e), URL: checkURL(e)})
		}
	}
	return r
}

type checkOutcome int

const (
	outcomePassed checkOutcome = iota
	outcomePending
	outcomeFailed
)

func outcome(e rollupEntry) checkOutcome {
	// StatusContext reports a single state field.
	if e.State != "" {
		switch e.State {
		case "SUCCESS", "EXPECTED":
			return outcomePassed
		case "PENDING":
			return outcomePending
		default: // FAILURE, ERROR
			return outcomeFailed
		}
	}
	// CheckRun is pending until COMPLETED, then judged on its conclusion.
	if e.Status != "COMPLETED" {
		return outcomePending
	}
	switch e.Conclusion {
	case "SUCCESS", "NEUTRAL", "SKIPPED":
		return outcomePassed
	default: // FAILURE, TIMED_OUT, CANCELLED, ACTION_REQUIRED, STARTUP_FAILURE
		return outcomeFailed
	}
}

func checkName(e rollupEntry) string {
	if e.Name != "" {
		return e.Name
	}
	return e.Context
}

func checkURL(e rollupEntry) string {
	if e.DetailsURL != "" {
		return e.DetailsURL
	}
	return e.TargetURL
}
