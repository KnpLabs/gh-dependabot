package cmd

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
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

func TestDefaultApprove(t *testing.T) {
	tests := []struct {
		checks dependabot.CheckState
		want   bool
	}{
		{dependabot.CheckNone, true},
		{dependabot.CheckPassing, true},
		{dependabot.CheckPending, true},
		{dependabot.CheckFailing, false},
	}
	for _, tt := range tests {
		t.Run(checksStatus(tt.checks), func(t *testing.T) {
			if got := defaultApprove(dependabot.PullRequest{Checks: tt.checks}); got != tt.want {
				t.Errorf("defaultApprove(%v) = %v, want %v", tt.checks, got, tt.want)
			}
		})
	}
}

func TestPrintSummary(t *testing.T) {
	prs := []dependabot.PullRequest{{Number: 1}, {Number: 4}, {Number: 5}, {Number: 7}}

	tests := []struct {
		name  string
		model interactiveModel
		want  string
	}{
		{
			name:  "no pull requests",
			model: interactiveModel{},
			want:  "No open dependabot pull requests found.\n",
		},
		{
			name:  "error prints nothing",
			model: interactiveModel{prs: prs, err: errors.New("boom")},
		},
		{
			name: "approve only",
			model: interactiveModel{
				prs:           prs,
				index:         len(prs),
				approved:      []int{1, 4},
				approveFailed: []prMergeFailure{{number: 5, err: errors.New("not allowed")}},
				skipped:       []int{7},
			},
			want: "\nApproved (2): #1, #4\nApprove failed (1):\n  #5: not allowed\nSkipped  (1): #7\n",
		},
		{
			name: "merge with rebases and early quit",
			model: interactiveModel{
				prs:               prs,
				index:             2,
				quitted:           true,
				mergeAfterApprove: true,
				approved:          []int{1, 4},
				merged:            []int{1},
				mergeFailed:       []prMergeFailure{{number: 4, err: errors.New("merge conflict: base branch was modified")}},
				rebased:           []int{4},
			},
			want: "\nApproved (2): #1, #4\nMerged   (1): #1\nMerge failed (1):\n  #4: merge conflict: base branch was modified\n" +
				"Rebased  (1): #4\nSkipped  (0): —\nUnreviewed (2): #5, #7\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			tt.model.printSummary(&out)
			if got := out.String(); got != tt.want {
				t.Errorf("printSummary() mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}
