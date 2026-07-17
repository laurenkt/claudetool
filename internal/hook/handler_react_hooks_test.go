package hook

import (
	"strings"
	"testing"
)

func TestReactHooksFlagsUseState(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/Panel.tsx",
		Content:  "const [x, setX] = React.useState(0)",
	})
	out := runHandlerOutput(t, "react-named-hooks", input)
	if out == nil || out.HookSpecificOutput == nil {
		t.Fatalf("want advisory output, got %+v", out)
	}
	if out.Decision == "block" {
		t.Errorf("decision = %q, want non-blocking", out.Decision)
	}
	ctx := out.HookSpecificOutput.AdditionalContext
	if !strings.Contains(ctx, "import React hooks by name") {
		t.Errorf("context should suggest named imports, got %q", ctx)
	}
	if !strings.Contains(ctx, "useState") {
		t.Errorf("context should name the offending hook, got %q", ctx)
	}
	if !strings.Contains(ctx, "React.useState(0)") {
		t.Errorf("context should quote the offending line, got %q", ctx)
	}
}

func TestReactHooksFlagsVariousHooks(t *testing.T) {
	for _, hook := range []string{
		"React.useEffect(() => {})",
		"React.useMemo(() => 1, [])",
		"React.useRef(null)",
		"React.useCallback(fn, [])",
		"React.useContext(Ctx)",
	} {
		input := makeToolInput("PostToolUse", "Edit", EditInput{
			FilePath:  "/src/x.ts",
			NewString: hook,
		})
		out := runHandlerOutput(t, "react-named-hooks", input)
		if out == nil {
			t.Errorf("want advisory for %q, got nil", hook)
		}
	}
}

func TestReactHooksIgnoresNamedCall(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.tsx",
		Content:  "const [x, setX] = useState(0)",
	})
	out := runHandlerOutput(t, "react-named-hooks", input)
	if out != nil {
		t.Errorf("want nil (already a named import), got %+v", out)
	}
}

func TestReactHooksIgnoresNonHookMembers(t *testing.T) {
	// React.FC / React.ReactNode are namespace types, not hook calls.
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.tsx",
		Content:  "const C: React.FC = () => null\ntype N = React.ReactNode",
	})
	out := runHandlerOutput(t, "react-named-hooks", input)
	if out != nil {
		t.Errorf("want nil (non-hook React members), got %+v", out)
	}
}

func TestReactHooksIgnoresNonReactUse(t *testing.T) {
	// A `.use(` that isn't a React hook (lowercase, or different receiver).
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.tsx",
		Content:  "app.use(middleware)\nReact.useable(x)",
	})
	out := runHandlerOutput(t, "react-named-hooks", input)
	if out != nil {
		t.Errorf("want nil (not a React.useXxx hook), got %+v", out)
	}
}

func TestReactHooksIgnoresNonJSFiles(t *testing.T) {
	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: "/src/x.go",
		Content:  "React.useState(0)",
	})
	out := runHandlerOutput(t, "react-named-hooks", input)
	if out != nil {
		t.Errorf("want nil (non-JS/TS file), got %+v", out)
	}
}

func TestReactHooksIgnoresNonWriteEdit(t *testing.T) {
	input := makeToolInput("PostToolUse", "Bash", BashInput{
		Command: "echo React.useState(0)",
	})
	out := runHandlerOutput(t, "react-named-hooks", input)
	if out != nil {
		t.Errorf("want nil (non-Write/Edit), got %+v", out)
	}
}

func TestReactHooksEditScansNewStringOnly(t *testing.T) {
	input := makeToolInput("PostToolUse", "Edit", EditInput{
		FilePath:  "/src/x.tsx",
		OldString: "React.useState(0)",
		NewString: "useState(0)",
	})
	out := runHandlerOutput(t, "react-named-hooks", input)
	if out != nil {
		t.Errorf("want nil (pattern only in OldString), got %+v", out)
	}
}
