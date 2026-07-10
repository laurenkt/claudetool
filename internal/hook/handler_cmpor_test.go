package hook

import (
	"strings"
	"testing"
)

func TestCmpOrFlagsLoopForm(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content: `package svc

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}`,
	})
	out := runHandlerOutput(t, "go-cmp-or", input)
	if out == nil || out.HookSpecificOutput == nil || out.HookSpecificOutput.AdditionalContext == "" {
		t.Fatalf("want advisory output, got %+v", out)
	}
	if out.Decision == "block" {
		t.Errorf("decision = %q, want non-blocking", out.Decision)
	}
	if !strings.Contains(out.HookSpecificOutput.AdditionalContext, "cmp.Or") {
		t.Errorf("additionalContext = %q, want mention of cmp.Or", out.HookSpecificOutput.AdditionalContext)
	}
}

func TestCmpOrFlagsChainForm(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content: `func pick(a, b, c string) string {
	if a != "" {
		return a
	}
	if b != "" {
		return b
	}
	return ""
}`,
	})
	out := runHandlerOutput(t, "go-cmp-or", input)
	if out == nil || out.HookSpecificOutput == nil {
		t.Fatalf("want advisory output, got %+v", out)
	}
}

func TestCmpOrFlagsInlineIf(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content: `func pick(a, b string) string {
	if a != "" { return a }
	if b != "" { return b }
	return ""
}`,
	})
	out := runHandlerOutput(t, "go-cmp-or", input)
	if out == nil || out.HookSpecificOutput == nil {
		t.Fatalf("want advisory output, got %+v", out)
	}
}

func TestCmpOrFlagsSelectorOperand(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content: `func name(u User) string {
	if u.Nickname != "" {
		return u.Nickname
	}
	return ""
}`,
	})
	out := runHandlerOutput(t, "go-cmp-or", input)
	if out == nil || out.HookSpecificOutput == nil {
		t.Fatalf("want advisory output for selector operand, got %+v", out)
	}
}

func TestCmpOrAllowsDefaultFallback(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content: `func name(s string) string {
	if s != "" {
		return s
	}
	return "anonymous"
}`,
	})
	out := runHandlerOutput(t, "go-cmp-or", input)
	if out != nil {
		t.Errorf("want nil (non-empty fallback is not cmp.Or), got %+v", out)
	}
}

func TestCmpOrAllowsGuardWithoutReturningGuarded(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content: `func lookup(key string) string {
	if key != "" {
		return cache[key]
	}
	return ""
}`,
	})
	out := runHandlerOutput(t, "go-cmp-or", input)
	if out != nil {
		t.Errorf("want nil (guard does not return the guarded value), got %+v", out)
	}
}

func TestCmpOrAllowsGuardWithoutEmptyFallback(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content: `func f(v string) {
	if v != "" {
		return v
	}
	doSomethingElse()
}`,
	})
	out := runHandlerOutput(t, "go-cmp-or", input)
	if out != nil {
		t.Errorf("want nil (no empty-string fallback), got %+v", out)
	}
}

func TestCmpOrIgnoresNonGo(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.py",
		Content: `if v != "":
	return v
return ""`,
	})
	out := runHandlerOutput(t, "go-cmp-or", input)
	if out != nil {
		t.Errorf("want nil (non-.go), got %+v", out)
	}
}

func TestCmpOrIgnoresNonWriteEdit(t *testing.T) {
	input := makeToolInput("PostToolUse", "Bash", BashInput{
		Command: `echo 'if v != "" { return v }; return ""'`,
	})
	out := runHandlerOutput(t, "go-cmp-or", input)
	if out != nil {
		t.Errorf("want nil (non-Write/Edit), got %+v", out)
	}
}

func TestCmpOrEditScansNewStringOnly(t *testing.T) {
	input := makeToolInput("PostToolUse", "Edit", EditInput{
		FilePath: "/src/x.go",
		OldString: `	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""`,
		NewString: `	return cmp.Or(values...)`,
	})
	out := runHandlerOutput(t, "go-cmp-or", input)
	if out != nil {
		t.Errorf("want nil (pattern only in OldString), got %+v", out)
	}
}
