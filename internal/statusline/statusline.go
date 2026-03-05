package statusline

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/laurenkt/claudetool/internal/ansi"
	"github.com/laurenkt/claudetool/internal/ghutil"
	"github.com/laurenkt/claudetool/internal/gitutil"
	"github.com/laurenkt/claudetool/internal/pathutil"
)

// Run reads Claude Code JSON from stdin and writes a formatted statusline to stdout.
func Run(stdin io.Reader, stdout io.Writer) error {
	var in Input
	if err := json.NewDecoder(stdin).Decode(&in); err != nil {
		return fmt.Errorf("decode stdin: %w", err)
	}

	var parts []string

	// Directory (bold, shortened)
	dir := in.Workspace.CurrentDir
	if dir == "" {
		dir = in.CWD
	}
	if dir != "" {
		parts = append(parts, ansi.Wrap(ansi.Bold, pathutil.Shorten(dir, 30)))
	}

	// Merged PR indicator + Git branch (cyan)
	var branch string
	if dir != "" {
		branch = gitutil.Branch(dir)
		if branch != "" && ghutil.BranchPRMerged(dir, branch) {
			parts = append(parts, "✅")
		}
		if branch != "" {
			parts = append(parts, ansi.Wrap(ansi.FgCyan, branch))
		}
	}

	// Model (magenta)
	if in.Model.DisplayName != "" {
		parts = append(parts, ansi.Wrap(ansi.FgMagenta, in.Model.DisplayName))
	}

	// Cost (green, hidden if < $0.01)
	parts = append(parts, ansi.Wrap(ansi.FgGreen, fmt.Sprintf("$%.2f", in.Cost.TotalCostUSD)))

	// Session duration (gray, hidden until populated)
	if in.Cost.TotalDurationMS > 0 {
		totalMin := int(in.Cost.TotalDurationMS / 60000)
		h, m := totalMin/60, totalMin%60
		var dur string
		if h > 0 {
			dur = fmt.Sprintf("%dh%dm", h, m)
		} else {
			dur = fmt.Sprintf("%dm", m)
		}
		parts = append(parts, ansi.Wrap(ansi.FgGray, dur))
	}

	// Context usage percentage
	pct := in.ContextWindow.UsedPercentage
	label := fmt.Sprintf("%.0f%%", pct)
	var style string
	switch {
	case pct > 50:
		style = ansi.Bold + ansi.FgRed
	case pct > 30:
		style = ansi.FgYellow
	default:
		style = ansi.FgGray
	}
	parts = append(parts, ansi.Wrap(style, label))

	fmt.Fprintln(stdout, strings.Join(parts, "  "))
	return nil
}
