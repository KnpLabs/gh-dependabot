package cmd

import (
	"testing"
	"unicode/utf8"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

func TestChecksStatus(t *testing.T) {
	tests := []struct {
		state dependabot.CheckState
		want  string
	}{
		{dependabot.CheckNone, "none"},
		{dependabot.CheckPassing, "✓ passing"},
		{dependabot.CheckFailing, "✗ failing"},
		{dependabot.CheckPending, "● pending"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := checksStatus(tt.state); got != tt.want {
				t.Errorf("checksStatus(%v) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestMergeableStatus(t *testing.T) {
	tests := []struct {
		mergeable dependabot.Mergeability
		want      string
	}{
		{dependabot.Mergeable, "✓ yes"},
		{dependabot.Conflicting, "✗ conflict"},
		{dependabot.Unknown, "● unknown"},
		{"", "● unknown"},
	}
	for _, tt := range tests {
		t.Run(string(tt.mergeable), func(t *testing.T) {
			if got := mergeableStatus(tt.mergeable); got != tt.want {
				t.Errorf("mergeableStatus(%q) = %q, want %q", tt.mergeable, got, tt.want)
			}
		})
	}
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		max   int
		want  string
	}{
		{"shorter than max", "Bump foo", 20, "Bump foo"},
		{"exact length", "Bump foo", 8, "Bump foo"},
		{"ascii truncated", "Bump foo from 1.0 to 2.0", 10, "Bump foo…"},
		{"trailing space trimmed", "Bump foo bar", 6, "Bump…"},
		{"utf-8 exact length in runes", "café", 4, "café"},
		{"utf-8 truncated", "Mise à jour de café", 8, "Mise à…"},
		{"emoji not split", "🚀 Bump lodash", 3, "🚀…"},
		{"multi-rune emoji kept whole", "⬆️ Bump lodash from 4.17.20 to 4.17.21", 12, "⬆️ Bump lod…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateTitle(tt.title, tt.max)
			if got != tt.want {
				t.Errorf("truncateTitle(%q, %d) = %q, want %q", tt.title, tt.max, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncateTitle(%q, %d) returned invalid UTF-8: %q", tt.title, tt.max, got)
			}
			if n := utf8.RuneCountInString(got); n > tt.max {
				t.Errorf("truncateTitle(%q, %d) is %d runes long, want at most %d", tt.title, tt.max, n, tt.max)
			}
		})
	}
}
