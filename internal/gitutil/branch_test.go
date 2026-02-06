package gitutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBranch(t *testing.T) {
	t.Run("normal branch", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, ".git")
		os.Mkdir(gitDir, 0o755)
		os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)

		got := Branch(dir)
		if got != "main" {
			t.Errorf("Branch() = %q, want %q", got, "main")
		}
	})

	t.Run("detached HEAD", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, ".git")
		os.Mkdir(gitDir, 0o755)
		os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("abc1234def5678\n"), 0o644)

		got := Branch(dir)
		if got != "abc1234" {
			t.Errorf("Branch() = %q, want %q", got, "abc1234")
		}
	})

	t.Run("worktree", func(t *testing.T) {
		dir := t.TempDir()
		// Create the "real" git dir somewhere else
		realGitDir := filepath.Join(dir, "real-git")
		os.MkdirAll(realGitDir, 0o755)
		os.WriteFile(filepath.Join(realGitDir, "HEAD"), []byte("ref: refs/heads/feature\n"), 0o644)

		// Worktree .git file points to the real git dir
		worktree := filepath.Join(dir, "worktree")
		os.MkdirAll(worktree, 0o755)
		os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+realGitDir+"\n"), 0o644)

		got := Branch(worktree)
		if got != "feature" {
			t.Errorf("Branch() = %q, want %q", got, "feature")
		}
	})

	t.Run("subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, ".git")
		os.Mkdir(gitDir, 0o755)
		os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/dev\n"), 0o644)
		sub := filepath.Join(dir, "a", "b")
		os.MkdirAll(sub, 0o755)

		got := Branch(sub)
		if got != "dev" {
			t.Errorf("Branch() = %q, want %q", got, "dev")
		}
	})

	t.Run("not a repo", func(t *testing.T) {
		dir := t.TempDir()
		got := Branch(dir)
		if got != "" {
			t.Errorf("Branch() = %q, want empty", got)
		}
	})
}
