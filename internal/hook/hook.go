package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Handler inspects a hook Input and returns an Output.
// Return nil to take no action (equivalent to exit 0 with no stdout).
// Return a non-nil error to signal exit 2 (blocking error via stderr).
type Handler func(*Input) (*Output, error)

// registry maps handler names to their implementations.
var registry = map[string]Handler{}

// Register adds a named handler. Call from init() in handler files.
// Handler names must not begin with '-' (reserved for flags).
func Register(name string, h Handler) {
	if strings.HasPrefix(name, "-") {
		panic(fmt.Sprintf("hook: handler name %q must not start with '-'", name))
	}
	registry[name] = h
}

// Run is the entry point for "claudetool hook <handler> [<handler>...] [flags]".
//
// Any positional arg not beginning with '-' is treated as a handler name; the
// remaining tokens (those beginning with '-' and any values following them)
// are passed through to every handler via Input.Args so flag-driven handlers
// like `dump -o <path>` keep working.
//
// Handlers run in the order given. The chain stops at the first handler that
// returns an error (exit 2) or an Output with Decision == "block". Otherwise
// non-blocking outputs are merged: additionalContext is concatenated; first
// non-empty wins for scalar decision fields.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 1
	}

	// Positional handler names come first; the first token starting with '-'
	// begins the flag tail, which is passed through to every handler.
	var names, flags []string
	for i, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = args[i:]
			break
		}
		names = append(names, a)
	}

	if len(names) == 0 {
		usage(stderr)
		return 1
	}

	handlersToRun := make([]Handler, 0, len(names))
	for _, name := range names {
		h, ok := registry[name]
		if !ok {
			fmt.Fprintf(stderr, "unknown hook handler: %s\n", name)
			usage(stderr)
			return 1
		}
		handlersToRun = append(handlersToRun, h)
	}

	raw, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "read stdin: %v\n", err)
		return 1
	}

	var input Input
	if err := json.Unmarshal(raw, &input); err != nil {
		fmt.Fprintf(stderr, "decode stdin: %v\n", err)
		return 1
	}
	input.RawJSON = raw
	input.Args = flags

	var merged *Output
	for i, h := range handlersToRun {
		out, err := h(&input)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 2
		}
		if out == nil {
			continue
		}
		if out.Decision == "block" {
			// Block short-circuits the chain. Emit and stop.
			if merr := json.NewEncoder(stdout).Encode(out); merr != nil {
				fmt.Fprintf(stderr, "encode output: %v\n", merr)
				return 1
			}
			return 0
		}
		merged = mergeOutputs(merged, out)
		_ = i
	}

	if merged == nil {
		return 0
	}
	if err := json.NewEncoder(stdout).Encode(merged); err != nil {
		fmt.Fprintf(stderr, "encode output: %v\n", err)
		return 1
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: claudetool hook <handler> [<handler>...] [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "available handlers:")
	for name := range registry {
		fmt.Fprintf(w, "  %s\n", name)
	}
}

// mergeOutputs combines a non-blocking output into the accumulator. Scalar
// fields are first-wins; additionalContext from HookSpecificOutput is concatenated.
func mergeOutputs(acc, next *Output) *Output {
	if acc == nil {
		// Copy so subsequent merges don't mutate a handler's return value.
		copy := *next
		if next.HookSpecificOutput != nil {
			hso := *next.HookSpecificOutput
			copy.HookSpecificOutput = &hso
		}
		return &copy
	}
	if acc.Continue == nil && next.Continue != nil {
		acc.Continue = next.Continue
	}
	if acc.StopReason == "" {
		acc.StopReason = next.StopReason
	}
	if next.SuppressOutput {
		acc.SuppressOutput = true
	}
	if acc.SystemMessage == "" {
		acc.SystemMessage = next.SystemMessage
	} else if next.SystemMessage != "" {
		acc.SystemMessage = acc.SystemMessage + "\n\n" + next.SystemMessage
	}
	if acc.Decision == "" {
		acc.Decision = next.Decision
		acc.Reason = next.Reason
	}
	if next.HookSpecificOutput != nil {
		if acc.HookSpecificOutput == nil {
			hso := *next.HookSpecificOutput
			acc.HookSpecificOutput = &hso
		} else {
			a := acc.HookSpecificOutput
			n := next.HookSpecificOutput
			if a.HookEventName == "" {
				a.HookEventName = n.HookEventName
			}
			if a.PermissionDecision == "" {
				a.PermissionDecision = n.PermissionDecision
				a.PermissionDecisionReason = n.PermissionDecisionReason
			}
			if a.UpdatedInput == nil {
				a.UpdatedInput = n.UpdatedInput
			}
			if a.AdditionalContext == "" {
				a.AdditionalContext = n.AdditionalContext
			} else if n.AdditionalContext != "" {
				a.AdditionalContext = a.AdditionalContext + "\n\n" + n.AdditionalContext
			}
		}
	}
	return acc
}

// handlers returns the names of all registered handlers.
func handlers() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
