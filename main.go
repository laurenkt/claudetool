package main

import (
	"fmt"
	"os"

	"github.com/laurenkt/claudetool/internal/hook"
	"github.com/laurenkt/claudetool/internal/statusline"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: claudetool <command>")
		fmt.Fprintln(os.Stderr, "commands: statusline, hook")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "statusline":
		if err := statusline.Run(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "hook":
		os.Exit(hook.Run(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
