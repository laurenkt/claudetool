package hook

import (
	"encoding/json"
	"fmt"
	"io"
)

// Handler inspects a hook Input and returns an Output.
// Return nil to take no action (equivalent to exit 0 with no stdout).
// Return a non-nil error to signal exit 2 (blocking error via stderr).
type Handler func(*Input) (*Output, error)

// registry maps handler names to their implementations.
var registry = map[string]Handler{}

// Register adds a named handler. Call from init() in handler files.
func Register(name string, h Handler) {
	registry[name] = h
}

// Run is the entry point for "claudetool hook <handler-name>".
// It reads JSON from stdin, looks up the handler, runs it, and writes
// the result to stdout (or stderr on error).
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: claudetool hook <handler>")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "available handlers:")
		for name := range registry {
			fmt.Fprintf(stderr, "  %s\n", name)
		}
		return 1
	}

	name := args[0]
	handler, ok := registry[name]
	if !ok {
		fmt.Fprintf(stderr, "unknown hook handler: %s\n", name)
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "available handlers:")
		for n := range registry {
			fmt.Fprintf(stderr, "  %s\n", n)
		}
		return 1
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
	input.Args = args[1:] // remaining args after handler name

	output, err := handler(&input)
	if err != nil {
		// exit 2 = blocking error; stderr is shown to Claude
		fmt.Fprintln(stderr, err.Error())
		return 2
	}

	if output == nil {
		return 0
	}

	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		fmt.Fprintf(stderr, "encode output: %v\n", err)
		return 1
	}
	return 0
}

// handlers returns the names of all registered handlers.
func handlers() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
