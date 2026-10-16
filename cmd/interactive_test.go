package cmd

import (
	"reflect"
	"testing"
)

func TestWrapWords(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  []string
	}{
		{"empty", "", 10, []string{""}},
		{"whitespace only", "   ", 10, []string{""}},
		{"fits on one line", "Bump foo", 10, []string{"Bump foo"}},
		{"exact width", "Bump foo", 8, []string{"Bump foo"}},
		{"wraps", "Bump foo from 1.0 to 2.0", 10, []string{"Bump foo", "from 1.0", "to 2.0"}},
		{"collapses spaces", "Bump   foo\tbar", 20, []string{"Bump foo bar"}},
		{"word longer than width", "Bump github.com/cli/go-gh/v2 now", 10, []string{"Bump", "github.com/cli/go-gh/v2", "now"}},
		{"counts runes not bytes", "café café", 9, []string{"café café"}},
		{"zero width returns input", "Bump foo", 0, []string{"Bump foo"}},
		{"negative width returns input", "Bump  foo", -1, []string{"Bump  foo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wrapWords(tt.s, tt.width); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("wrapWords(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

func TestJoinNumbers(t *testing.T) {
	tests := []struct {
		name string
		nums []int
		want string
	}{
		{"nil", nil, ""},
		{"empty", []int{}, ""},
		{"one", []int{42}, "#42"},
		{"several keep order", []int{3, 1, 2}, "#3, #1, #2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinNumbers(tt.nums); got != tt.want {
				t.Errorf("joinNumbers(%v) = %q, want %q", tt.nums, got, tt.want)
			}
		})
	}
}
