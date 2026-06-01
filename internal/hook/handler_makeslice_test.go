package hook

import (
	"strings"
	"testing"
)

func TestMakeSliceFlagsLenCap(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/service/timeline.go",
		Content: `package svc

func convert(events []Event) []dto.PayPortalTimelineEvent {
	out := make([]dto.PayPortalTimelineEvent, 0, len(events))
	for _, event := range events {
		out = append(out, bizumTimelineEvent(event))
	}
	return out
}`,
	})
	out := runHandlerOutput(t, "go-makeslice", input)
	if out == nil {
		t.Fatal("want advisory output, got nil")
	}
	if out.Decision == "block" {
		t.Errorf("decision = %q, want non-blocking", out.Decision)
	}
	if out.HookSpecificOutput == nil || out.HookSpecificOutput.AdditionalContext == "" {
		t.Fatalf("want additionalContext, got %+v", out.HookSpecificOutput)
	}
	if !strings.Contains(out.HookSpecificOutput.AdditionalContext, "var x []T") {
		t.Errorf("additionalContext = %q, want guidance about var x []T", out.HookSpecificOutput.AdditionalContext)
	}
	if !strings.Contains(out.HookSpecificOutput.AdditionalContext, "make([]dto.PayPortalTimelineEvent") {
		t.Errorf("additionalContext should quote the offending line, got %q", out.HookSpecificOutput.AdditionalContext)
	}
}

func TestMakeSliceFlagsIntCap(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content:  `out := make([]int, 0, 100)`,
	})
	out := runHandlerOutput(t, "go-makeslice", input)
	if out == nil || out.HookSpecificOutput == nil {
		t.Fatalf("want advisory output, got %+v", out)
	}
}

func TestMakeSliceFlagsNestedCap(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content:  `xs := make([]string, 0, len(a)+len(b))`,
	})
	out := runHandlerOutput(t, "go-makeslice", input)
	if out == nil || out.HookSpecificOutput == nil {
		t.Fatalf("want advisory output, got %+v", out)
	}
}

func TestMakeSliceAllowsNonZeroLen(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content:  `out := make([]int, 5)`,
	})
	out := runHandlerOutput(t, "go-makeslice", input)
	if out != nil {
		t.Errorf("want nil (no zero-length-with-cap pattern), got %+v", out)
	}
}

func TestMakeSliceAllowsLenAndCap(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content:  `out := make([]int, n, 2*n)`,
	})
	out := runHandlerOutput(t, "go-makeslice", input)
	if out != nil {
		t.Errorf("want nil (non-zero length is fine), got %+v", out)
	}
}

func TestMakeSliceAllowsMap(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content:  `m := make(map[string]int, 0)`,
	})
	out := runHandlerOutput(t, "go-makeslice", input)
	if out != nil {
		t.Errorf("want nil (map, not slice), got %+v", out)
	}
}

func TestMakeSliceIgnoresNonGo(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.py",
		Content:  `out := make([]int, 0, 10)`,
	})
	out := runHandlerOutput(t, "go-makeslice", input)
	if out != nil {
		t.Errorf("want nil (non-.go), got %+v", out)
	}
}

func TestMakeSliceIgnoresNonWriteEdit(t *testing.T) {
	input := makeToolInput("PostToolUse", "Bash", BashInput{
		Command: `echo "make([]int, 0, 10)"`,
	})
	out := runHandlerOutput(t, "go-makeslice", input)
	if out != nil {
		t.Errorf("want nil (non-Write/Edit), got %+v", out)
	}
}

func TestMakeSliceEditScansNewStringOnly(t *testing.T) {
	input := makeToolInput("PostToolUse", "Edit", EditInput{
		FilePath:  "/src/x.go",
		OldString: `out := make([]int, 0, 10)`,
		NewString: `var out []int`,
	})
	out := runHandlerOutput(t, "go-makeslice", input)
	if out != nil {
		t.Errorf("want nil (pattern only in OldString), got %+v", out)
	}
}
