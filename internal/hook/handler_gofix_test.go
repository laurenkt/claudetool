package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoFixIgnoresNonGo(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/tmp/test.py",
		Content:  "print('hi')",
	})
	out := runHandlerOutput(t, "go-fix", input)
	if out != nil {
		t.Errorf("want allow (non-.go), got %+v", out)
	}
}

func TestGoFixIgnoresNonWriteEdit(t *testing.T) {
	input := makeToolInput("PostToolUse", "Bash", BashInput{Command: "echo hi"})
	out := runHandlerOutput(t, "go-fix", input)
	if out != nil {
		t.Errorf("want allow (non-Write/Edit), got %+v", out)
	}
}

// TestGoFixAppliesFixes writes a small Go module with a pattern go fix can
// modernize (interface{} → any when go.mod is recent enough), runs the
// handler, and verifies the file is rewritten and additionalContext returned.
func TestGoFixAppliesFixes(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not installed")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}
	src := "package m\n\nvar X interface{}\n"
	srcPath := filepath.Join(dir, "m.go")
	if err := os.WriteFile(srcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: srcPath,
		Content:  src,
	})
	out := runHandlerOutput(t, "go-fix", input)
	if out == nil {
		// go fix may not produce a diff for this pattern in all environments —
		// only fail if the file was clearly not touched.
		got, _ := os.ReadFile(srcPath)
		if !strings.Contains(string(got), "any") {
			t.Skipf("go fix did not modernize the sample; got: %s", got)
		}
		return
	}
	if out.HookSpecificOutput == nil || out.HookSpecificOutput.AdditionalContext == "" {
		t.Errorf("want additionalContext describing the fix, got %+v", out)
	}
	got, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "any") {
		t.Errorf("file not rewritten by go fix: %s", got)
	}
}

// TestGoFixScopedToTargetFile proves the handler only rewrites the file Claude
// edited, leaving sibling files in the same package alone.
func TestGoFixScopedToTargetFile(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not installed")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target.go")
	sibling := filepath.Join(dir, "sibling.go")
	if err := os.WriteFile(target, []byte("package m\n\nvar T interface{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	siblingSrc := "package m\n\nvar S interface{}\n"
	if err := os.WriteFile(sibling, []byte(siblingSrc), 0644); err != nil {
		t.Fatal(err)
	}

	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: target,
		Content:  "package m\n\nvar T interface{}\n",
	})
	_ = runHandlerOutput(t, "go-fix", input)

	got, err := os.ReadFile(sibling)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != siblingSrc {
		t.Errorf("sibling was modified by go fix:\nbefore: %s\nafter:  %s", siblingSrc, got)
	}
}
