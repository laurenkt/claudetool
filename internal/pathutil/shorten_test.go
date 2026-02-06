package pathutil

import (
	"os"
	"testing"
)

func TestShorten(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name   string
		path   string
		maxLen int
		want   string
	}{
		{"short path unchanged", "/tmp", 50, "/tmp"},
		{"home replaced with tilde", home, 50, "~"},
		{"home subdir replaced", home + "/projects/foo", 50, "~/projects/foo"},
		{"long path shortened", "~/src/github.com/laurenkt/claudetool", 20, "~/s/g/l/claudetool"},
		{"already short enough", "~/foo/bar", 50, "~/foo/bar"},
		{"two parts only", "~/foo", 1, "~/foo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Shorten(tt.path, tt.maxLen)
			if got != tt.want {
				t.Errorf("Shorten(%q, %d) = %q, want %q", tt.path, tt.maxLen, got, tt.want)
			}
		})
	}
}
