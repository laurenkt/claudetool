package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDumpWritesFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "dump.json")
	input := makeToolInput("PreToolUse", "Bash", BashInput{Command: "ls"})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dump", "-o", out}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read dump file: %v", err)
	}
	if !strings.Contains(string(data), `"hook_event_name":"PreToolUse"`) {
		t.Errorf("dump = %s, want hook_event_name", string(data))
	}
	if !strings.Contains(string(data), `"tool_name":"Bash"`) {
		t.Errorf("dump = %s, want tool_name", string(data))
	}
}

func TestDumpNoFlag(t *testing.T) {
	input := makeToolInput("PreToolUse", "Bash", BashInput{Command: "ls"})
	code, stderr := runHandler(t, "dump", input)
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (missing -o flag); stderr: %s", code, stderr)
	}
}

func TestDumpAppends(t *testing.T) {
	out := filepath.Join(t.TempDir(), "dump.json")

	input1 := makeToolInput("PreToolUse", "Bash", BashInput{Command: "ls"})
	input2 := makeToolInput("PreToolUse", "Write", WriteInput{FilePath: "/tmp/x", Content: "y"})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dump", "-o", out}, strings.NewReader(input1), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("first dump: exit code = %d; stderr: %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"dump", "-o", out}, strings.NewReader(input2), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second dump: exit code = %d; stderr: %s", code, stderr.String())
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if !strings.Contains(lines[0], `"Bash"`) {
		t.Errorf("line 0 = %s, want Bash", lines[0])
	}
	if !strings.Contains(lines[1], `"Write"`) {
		t.Errorf("line 1 = %s, want Write", lines[1])
	}
}

func TestDumpDoesNotBlock(t *testing.T) {
	out := filepath.Join(t.TempDir(), "dump.json")
	input := makeToolInput("PreToolUse", "Bash", BashInput{Command: "ls"})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"dump", "-o", out}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// No output on stdout means no decision — Claude proceeds normally
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty (no-op)", stdout.String())
	}
}
