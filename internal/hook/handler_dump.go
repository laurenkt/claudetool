package hook

import (
	"fmt"
	"os"
)

func init() {
	Register("dump", handleDump)
}

// handleDump writes the raw hook input JSON to a file for debugging.
// Usage: claudetool hook dump -o <filepath>
func handleDump(in *Input) (*Output, error) {
	path, err := parseDumpFlag(in.Args)
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %v", path, err)
	}
	defer f.Close()

	// Write JSON followed by newline so each invocation is on its own line
	if _, err := f.Write(in.RawJSON); err != nil {
		return nil, fmt.Errorf("write %s: %v", path, err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		return nil, fmt.Errorf("write %s: %v", path, err)
	}
	return nil, nil
}

func parseDumpFlag(args []string) (string, error) {
	for i, arg := range args {
		if arg == "-o" && i+1 < len(args) {
			return args[i+1], nil
		}
	}
	return "", fmt.Errorf("usage: claudetool hook dump -o <filepath>")
}
