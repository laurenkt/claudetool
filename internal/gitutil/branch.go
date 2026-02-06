package gitutil

import (
	"os"
	"path/filepath"
	"strings"
)

// Branch returns the current git branch name for the given directory.
// It walks up the directory tree to find .git, then reads HEAD directly.
// Returns "" if not in a git repo or on error.
func Branch(dir string) string {
	gitDir := findGitDir(dir)
	if gitDir == "" {
		return ""
	}
	return readHead(gitDir)
}

func findGitDir(dir string) string {
	for {
		candidate := filepath.Join(dir, ".git")
		info, err := os.Lstat(candidate)
		if err == nil {
			if info.IsDir() {
				return candidate
			}
			// Worktree: .git is a file containing "gitdir: <path>"
			data, err := os.ReadFile(candidate)
			if err != nil {
				return ""
			}
			line := strings.TrimSpace(string(data))
			if strings.HasPrefix(line, "gitdir: ") {
				return strings.TrimPrefix(line, "gitdir: ")
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func readHead(gitDir string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(data))
	if strings.HasPrefix(head, "ref: refs/heads/") {
		return strings.TrimPrefix(head, "ref: refs/heads/")
	}
	// Detached HEAD - return short SHA
	if len(head) >= 7 {
		return head[:7]
	}
	return head
}
