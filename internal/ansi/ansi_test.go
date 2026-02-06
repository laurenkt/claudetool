package ansi

import "testing"

func TestWrap(t *testing.T) {
	tests := []struct {
		name  string
		style string
		s     string
		want  string
	}{
		{"empty string", FgCyan, "", ""},
		{"cyan text", FgCyan, "hello", "\033[36mhello\033[0m"},
		{"bold text", Bold, "world", "\033[1mworld\033[0m"},
		{"combined styles", Bold + FgRed, "err", "\033[1m\033[31merr\033[0m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.style, tt.s)
			if got != tt.want {
				t.Errorf("Wrap(%q, %q) = %q, want %q", tt.style, tt.s, got, tt.want)
			}
		})
	}
}
