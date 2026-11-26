package cmd

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

func TestChecksStatus(t *testing.T) {
	tests := []struct {
		state dependabot.CheckState
		want  string
		color string
	}{
		{dependabot.CheckNone, "none", "30"},
		{dependabot.CheckPassing, "✓ passing", "32"},
		{dependabot.CheckFailing, "✗ failing", "31"},
		{dependabot.CheckPending, "● pending", "30"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := checksStatus(tt.state, false); ansi.Strip(got) != tt.want || !strings.HasPrefix(got, "\x1b["+tt.color+"m") {
				t.Errorf("checksStatus(%v) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestMergeableStatus(t *testing.T) {
	tests := []struct {
		mergeable dependabot.Mergeability
		want      string
		color     string
	}{
		{dependabot.Mergeable, "✓ yes", "32"},
		{dependabot.Conflicting, "✗ conflict", "31"},
		{dependabot.Unknown, "● unknown", "30"},
		{"", "● unknown", "30"},
	}
	for _, tt := range tests {
		t.Run(string(tt.mergeable), func(t *testing.T) {
			if got := mergeableStatus(tt.mergeable, false); ansi.Strip(got) != tt.want || !strings.HasPrefix(got, "\x1b["+tt.color+"m") {
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
