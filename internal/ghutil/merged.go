package ghutil

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

// BranchPRMerged returns true if the given branch has a merged PR on GitHub.
// Returns false on any error (no gh, no PR, timeout, etc).
func BranchPRMerged(repoDir, branch string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "pr", "view", branch, "--json", "state")
	cmd.Dir = repoDir

	out, err := cmd.Output()
	if err != nil {
		return false
	}

	var result struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return false
	}
	return result.State == "MERGED"
}
