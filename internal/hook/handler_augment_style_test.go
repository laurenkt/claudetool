package hook

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAugmentStyleBlocksWrite(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/service.go",
		Content:  `return terrors.Augment(err, "failed to read database", nil)`,
	})
	code, stderr := runHandler(t, "go-augment-style", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (structured block)", code)
	}
	if !strings.Contains(stderr, "") {
		// block comes via stdout as Output with Decision="block", not stderr
	}
	// Verify structured output
	out := runHandlerOutput(t, "go-augment-style", input)
	if out.Decision != "block" {
		t.Errorf("decision = %q, want block", out.Decision)
	}
	if !strings.Contains(out.Reason, "failed to") {
		t.Errorf("reason = %q, want to contain matched text", out.Reason)
	}
}

func TestAugmentStyleAllowsCorrectStyle(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/service.go",
		Content:  `return terrors.Augment(err, "read database", nil)`,
	})
	out := runHandlerOutput(t, "go-augment-style", input)
	if out != nil {
		t.Errorf("output = %+v, want nil (allow)", out)
	}
}

func TestAugmentStyleAllBadPrefixes(t *testing.T) {
	prefixes := []string{
		"failed to ",
		"error ",
		"unable to ",
		"could not ",
		"couldn't ",
	}
	for _, prefix := range prefixes {
		t.Run(prefix, func(t *testing.T) {
			input := makeToolInput("PostToolUse", "Write", WriteInput{
				FilePath: "/src/service.go",
				Content:  `return terrors.Augment(err, "` + prefix + `do thing", nil)`,
			})
			out := runHandlerOutput(t, "go-augment-style", input)
			if out == nil || out.Decision != "block" {
				t.Errorf("prefix %q: want block, got %+v", prefix, out)
			}
		})
	}
}

func TestAugmentStyleIgnoresNonGo(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/service.py",
		Content:  `terrors.Augment(err, "failed to read database", nil)`,
	})
	out := runHandlerOutput(t, "go-augment-style", input)
	if out != nil {
		t.Errorf("output = %+v, want nil (skip non-.go)", out)
	}
}

func TestAugmentStyleEditScansNewStringOnly(t *testing.T) {
	// OldString has the bad pattern, NewString is clean — should allow
	input := makeToolInput("PostToolUse", "Edit", EditInput{
		FilePath:  "/src/service.go",
		OldString: `return terrors.Augment(err, "failed to read database", nil)`,
		NewString: `return terrors.Augment(err, "read database", nil)`,
	})
	out := runHandlerOutput(t, "go-augment-style", input)
	if out != nil {
		t.Errorf("output = %+v, want nil (old pattern in OldString only)", out)
	}

	// NewString has the bad pattern — should block
	input2 := makeToolInput("PostToolUse", "Edit", EditInput{
		FilePath:  "/src/service.go",
		OldString: `return terrors.Augment(err, "read database", nil)`,
		NewString: `return terrors.Augment(err, "failed to read database", nil)`,
	})
	out2 := runHandlerOutput(t, "go-augment-style", input2)
	if out2 == nil || out2.Decision != "block" {
		t.Errorf("output = %+v, want block", out2)
	}
}

func TestAugmentStyleAllowsNoAugment(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/service.go",
		Content:  `return fmt.Errorf("failed to read database: %w", err)`,
	})
	out := runHandlerOutput(t, "go-augment-style", input)
	if out != nil {
		t.Errorf("output = %+v, want nil (no terrors.Augment)", out)
	}
}

// runHandlerOutput runs a handler and returns the parsed Output (nil if no output).
func runHandlerOutput(t *testing.T, name, input string) *Output {
	t.Helper()
	var stdout, stderr strings.Builder
	code := Run([]string{name}, strings.NewReader(input), &stdout, &stderr)
	if code == 2 {
		t.Fatalf("unexpected exit 2 (stderr error): %s", stderr.String())
	}
	if code != 0 {
		t.Fatalf("unexpected exit %d: stderr=%s", code, stderr.String())
	}
	if stdout.Len() == 0 {
		return nil
	}
	var out Output
	if err := json.Unmarshal([]byte(stdout.String()), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	return &out
}
