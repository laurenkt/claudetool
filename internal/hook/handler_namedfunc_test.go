package hook

import (
	"strings"
	"testing"
)

func TestNamedFuncFlagsShortVarDecl(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/svc/payportal.go",
		Content: `package svc

func render(p Payment) []dto.Item {
	var items []dto.Item
	addCopyable := func(id, label, value string) {
		if value == "" {
			return
		}
		items = append(items, dto.Copyable{ID: id, Label: label, Value: value})
	}
	addCopyable("idempotency-key", "Idempotency key", p.IdempotencyKey)
	return items
}`,
	})
	out := runHandlerOutput(t, "go-named-func", input)
	if out == nil {
		t.Fatal("want advisory output, got nil")
	}
	if out.Decision == "block" {
		t.Errorf("decision = %q, want non-blocking", out.Decision)
	}
	if out.HookSpecificOutput == nil || out.HookSpecificOutput.AdditionalContext == "" {
		t.Fatalf("want additionalContext, got %+v", out.HookSpecificOutput)
	}
	if !strings.Contains(out.HookSpecificOutput.AdditionalContext, "addCopyable") {
		t.Errorf("additionalContext should quote the offending line, got %q", out.HookSpecificOutput.AdditionalContext)
	}
}

func TestNamedFuncFlagsAssignment(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content: `var fn func(int) int
fn = func(x int) int { return x + 1 }`,
	})
	out := runHandlerOutput(t, "go-named-func", input)
	if out == nil || out.HookSpecificOutput == nil {
		t.Fatalf("want advisory output, got %+v", out)
	}
}

func TestNamedFuncFlagsVarDecl(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content:  `var greet = func(name string) string { return "hi " + name }`,
	})
	out := runHandlerOutput(t, "go-named-func", input)
	if out == nil || out.HookSpecificOutput == nil {
		t.Fatalf("want advisory output, got %+v", out)
	}
}

func TestNamedFuncAllowsArgumentLiteral(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content: `slices.SortFunc(xs, func(a, b X) int {
	return a.ID - b.ID
})`,
	})
	out := runHandlerOutput(t, "go-named-func", input)
	if out != nil {
		t.Errorf("want nil (function literal as call argument), got %+v", out)
	}
}

func TestNamedFuncAllowsResultOfCall(t *testing.T) {
	// The RHS is a call returning []X — the name binds to a slice, not a func.
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content:  `results := slices.Filter(xs, func(x X) bool { return x.OK })`,
	})
	out := runHandlerOutput(t, "go-named-func", input)
	if out != nil {
		t.Errorf("want nil (RHS is a call, not a func literal), got %+v", out)
	}
}

func TestNamedFuncAllowsTopLevelFunc(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content:  `func addCopyable(id, label, value string) {}`,
	})
	out := runHandlerOutput(t, "go-named-func", input)
	if out != nil {
		t.Errorf("want nil (real func decl), got %+v", out)
	}
}

func TestNamedFuncSkipsTestFiles(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x_test.go",
		Content:  `assertEqual := func(a, b int) { /* ... */ }`,
	})
	out := runHandlerOutput(t, "go-named-func", input)
	if out != nil {
		t.Errorf("want nil (_test.go is exempt), got %+v", out)
	}
}

func TestNamedFuncIgnoresNonGo(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.py",
		Content:  `addCopyable := func(id string) {}`,
	})
	out := runHandlerOutput(t, "go-named-func", input)
	if out != nil {
		t.Errorf("want nil (non-.go), got %+v", out)
	}
}

func TestNamedFuncIgnoresNonWriteEdit(t *testing.T) {
	input := makeToolInput("PostToolUse", "Bash", BashInput{
		Command: `echo "addCopyable := func() {}"`,
	})
	out := runHandlerOutput(t, "go-named-func", input)
	if out != nil {
		t.Errorf("want nil (non-Write/Edit), got %+v", out)
	}
}

func TestNamedFuncEditScansNewStringOnly(t *testing.T) {
	input := makeToolInput("PostToolUse", "Edit", EditInput{
		FilePath:  "/src/x.go",
		OldString: `addCopyable := func(id string) {}`,
		NewString: `func addCopyable(id string) {}`,
	})
	out := runHandlerOutput(t, "go-named-func", input)
	if out != nil {
		t.Errorf("want nil (pattern only in OldString), got %+v", out)
	}
}
