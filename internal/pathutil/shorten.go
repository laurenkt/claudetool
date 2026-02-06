package pathutil

import (
	"os"
	"path/filepath"
	"strings"
)

// Shorten replaces $HOME with ~ and shortens intermediate directory names
// to their first character if the resulting path exceeds maxLen.
func Shorten(path string, maxLen int) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if path == home {
			return "~"
		}
		if strings.HasPrefix(path, home+"/") {
			path = "~" + path[len(home):]
		}
	}

	if len(path) <= maxLen {
		return path
	}

	parts := strings.Split(path, string(filepath.Separator))
	// Keep the first part (~ or root) and the last part intact, shorten the middle.
	if len(parts) <= 2 {
		return path
	}
	for i := 1; i < len(parts)-1; i++ {
		if len(parts[i]) > 0 {
			parts[i] = string([]rune(parts[i])[0])
		}
	}
	return strings.Join(parts, string(filepath.Separator))
}
