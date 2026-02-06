package ansi

const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	FgRed     = "\033[31m"
	FgGreen   = "\033[32m"
	FgYellow  = "\033[33m"
	FgMagenta = "\033[35m"
	FgCyan    = "\033[36m"
	FgGray    = "\033[90m"
)

// Wrap returns s wrapped in the given ANSI style, followed by a reset.
func Wrap(style, s string) string {
	if s == "" {
		return ""
	}
	return style + s + Reset
}
