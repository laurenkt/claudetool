package hook

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	Register("go-fix", handleGoFix)
}

// handleGoFix runs `go fix` on the .go file Claude just wrote or edited.
// If the file is modified, the unified diff is returned as additionalContext
// so Claude sees what was modernized.
//
// Use as a PostToolUse hook with matcher "Write|Edit".
func handleGoFix(in *Input) (*Output, error) {
	if in.ToolName != "Write" && in.ToolName != "Edit" {
		return nil, nil
	}

	filePath := extractFilePath(in)
	if filePath == "" || !strings.HasSuffix(filePath, ".go") {
		return nil, nil
	}

	dir, base := filepath.Split(filePath)

	before, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil
	}

	cmd := exec.Command("go", "fix", base)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		// go fix not available, build broken, etc. — best effort, don't block.
		return nil, nil
	}

	after, err := os.ReadFile(filePath)
	if err != nil || bytes.Equal(before, after) {
		return nil, nil
	}

	return &Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName: in.HookEventName,
			AdditionalContext: fmt.Sprintf(
				"go fix modernized %s. The following changes were applied automatically:\n\n%s",
				filePath, unifiedDiff(filePath, before, after),
			),
		},
	}, nil
}

// unifiedDiff returns a `diff -u` style patch. Falls back to a plain
// "before/after" dump if /usr/bin/diff isn't on PATH.
func unifiedDiff(path string, before, after []byte) string {
	tmp, err := os.CreateTemp("", "gofix-before-*.go")
	if err != nil {
		return fmt.Sprintf("--- before ---\n%s\n--- after ---\n%s", before, after)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(before); err != nil {
		tmp.Close()
		return fmt.Sprintf("--- before ---\n%s\n--- after ---\n%s", before, after)
	}
	tmp.Close()

	cmd := exec.Command("diff", "-u",
		"--label", path+" (before)",
		"--label", path+" (after)",
		tmp.Name(), path,
	)
	out, _ := cmd.Output() // exit 1 means "files differ" — that's expected.
	if len(out) == 0 {
		return fmt.Sprintf("--- before ---\n%s\n--- after ---\n%s", before, after)
	}
	return string(out)
}
