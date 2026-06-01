package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(nil, strings.NewReader("{}"), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Errorf("stderr = %q, want usage message", stderr.String())
	}
}

func TestRunUnknownHandler(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"nonexistent"}, strings.NewReader("{}"), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown hook handler") {
		t.Errorf("stderr = %q, want unknown handler message", stderr.String())
	}
}

func TestRunInvalidJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"no-cd"}, strings.NewReader("not json"), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestNoCDAllows(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"simple ls", "ls -la"},
		{"npm test", "npm test"},
		{"echo cd", "echo cd /tmp"},
		{"cdata var", "echo $cdata"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: tt.command})
			code, _ := runHandler(t, "no-cd", input)
			if code != 0 {
				t.Errorf("exit code = %d, want 0 (allow)", code)
			}
		})
	}
}

func TestNoCDBlocks(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"bare cd", "cd"},
		{"cd with path", "cd /tmp"},
		{"cd in chain", "ls && cd /tmp"},
		{"cd after semicolon", "echo hi; cd /tmp"},
		{"cd on second line", "echo hi\ncd /tmp"},
		{"cd after pipe", "echo hi | cd /tmp"},
		{"cd after or", "false || cd /tmp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: tt.command})
			code, stderr := runHandler(t, "no-cd", input)
			if code != 2 {
				t.Errorf("exit code = %d, want 2 (block)", code)
			}
			if !strings.Contains(stderr, "cd commands are not allowed") {
				t.Errorf("stderr = %q, want cd blocked message", stderr)
			}
		})
	}
}

func TestNoCDIgnoresNonBash(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", map[string]string{
		"file_path": "/tmp/test.txt",
		"content":   "cd /tmp",
	})
	code, _ := runHandler(t, "no-cd", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (ignore non-Bash)", code)
	}
}

func TestRedirectWritesNoEnv(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/old/generated/file.go",
		Content:  "package main",
	})
	code, _ := runHandler(t, "redirect-writes", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (no-op without env)", code)
	}
}

func TestRedirectWritesRedirects(t *testing.T) {
	t.Setenv("REDIRECT_FROM", "/old/generated")
	t.Setenv("REDIRECT_TO", "/new/generated")

	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/old/generated/file.go",
		Content:  "package main",
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"redirect-writes"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	var out Output
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.HookSpecificOutput == nil {
		t.Fatal("hookSpecificOutput is nil")
	}
	if out.HookSpecificOutput.PermissionDecision != "ask" {
		t.Errorf("permissionDecision = %q, want ask", out.HookSpecificOutput.PermissionDecision)
	}
	if out.HookSpecificOutput.UpdatedInput == nil {
		t.Fatal("updatedInput is nil")
	}
	fp, ok := out.HookSpecificOutput.UpdatedInput["file_path"].(string)
	if !ok || fp != "/new/generated/file.go" {
		t.Errorf("updatedInput.file_path = %q, want /new/generated/file.go", fp)
	}
}

func TestRedirectWritesNoMatch(t *testing.T) {
	t.Setenv("REDIRECT_FROM", "/old/generated")
	t.Setenv("REDIRECT_TO", "/new/generated")

	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/other/path/file.go",
		Content:  "package main",
	})
	code, _ := runHandler(t, "redirect-writes", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (no match)", code)
	}
}

func TestRunMultiHandlerStopsAtBlock(t *testing.T) {
	// go-augment-style blocks on the bad prefix; if it didn't stop the chain
	// we'd carry on to backend101 and never emit the block. Verify the first
	// blocking handler short-circuits.
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/svc/x.go",
		Content:  `terrors.Augment(err, "` + `failed to read account", nil)`,
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"go-augment-style", "backend101"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	var out Output
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v; stdout=%s", err, stdout.String())
	}
	if out.Decision != "block" {
		t.Errorf("decision = %q, want block", out.Decision)
	}
}

func TestRunMultiHandlerAllAllow(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/svc/x.txt",
		Content:  "harmless",
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"go-augment-style", "backend101"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunFlagsPassedToHandler(t *testing.T) {
	// `dump` reads -o from in.Args. Mix with another handler to prove the
	// flag tail is shared.
	out := filepath.Join(t.TempDir(), "dump.json")
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/svc/x.txt",
		Content:  "ok",
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"dump", "backend101", "-o", out}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read dump file: %v", err)
	}
	if !strings.Contains(string(data), `"tool_name":"Write"`) {
		t.Errorf("dump = %s, want tool_name", string(data))
	}
}

func TestRegisterPanicsOnDashPrefix(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("want panic on handler name starting with '-'")
		}
	}()
	Register("-bad", func(*Input) (*Output, error) { return nil, nil })
}

func TestHandlersRegistered(t *testing.T) {
	names := handlers()
	want := map[string]bool{
		"no-cd":              true,
		"no-diff-master":     true,
		"use-linear-mcp":     true,
		"go-swallowed-error": true,
		"go-augment-style":   true,
		"redirect-writes":    true,
		"dump":               true,
		"backend101":         true,
		"go-fix":             true,
	}
	for _, n := range names {
		delete(want, n)
	}
	for n := range want {
		t.Errorf("handler %q not registered", n)
	}
}

// helpers

func makeToolInput(event, tool string, toolInput any) string {
	ti, _ := json.Marshal(toolInput)
	input := Input{
		HookEventName: event,
		ToolName:      tool,
		ToolInput:     ti,
	}
	data, _ := json.Marshal(input)
	return string(data)
}

func runHandler(t *testing.T, name, input string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run([]string{name}, strings.NewReader(input), &stdout, &stderr)
	return code, stderr.String()
}
