package cmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/knplabs/gh-dependabot/internal/github"
)

func TestMergeCommand(t *testing.T) {
	errConflict := fmt.Errorf("%w: Pull request is not mergeable: the merge commit cannot be cleanly created.", github.ErrConflict)

	tests := []struct {
		name       string
		args       []string
		mergeErr   map[int]error
		rebaseErr  map[int]error
		wantStdout string
		wantStderr string
		wantCalls  []string
	}{
		{
			name:       "merges every eligible pull request",
			args:       []string{"merge", "--method", "squash"},
			wantStdout: "Merging PR #1...\nPR #1 merged.\nMerging PR #4...\nPR #4 merged.\n",
			wantCalls: []string{
				"AllowedMergeMethods",
				"ListOpenDependabotPRs",
				"Merge 1 squash deleteBranch=true",
				"Merge 4 squash deleteBranch=true",
			},
		},
		{
			name:       "uses the merge method and keeps the branch when asked to",
			args:       []string{"merge", "1", "--delete-branch=false"},
			wantStdout: "Merging PR #1...\nPR #1 merged.\n",
			wantCalls: []string{
				"AllowedMergeMethods",
				"ListOpenDependabotPRs",
				"Merge 1 merge deleteBranch=false",
			},
		},
		{
			name:       "skips pull requests that are not eligible",
			args:       []string{"merge", "3"},
			wantStdout: "No eligible dependabot pull requests found.\n",
			wantCalls:  []string{"AllowedMergeMethods", "ListOpenDependabotPRs"},
		},
		{
			name:       "asks dependabot to rebase on conflict",
			args:       []string{"merge", "--method", "rebase"},
			mergeErr:   map[int]error{1: errConflict},
			wantStdout: "Merging PR #1...\nPR #1 has conflicts, asked Dependabot to rebase it.\nMerging PR #4...\nPR #4 merged.\n",
			wantStderr: "Failed to merge PR #1: merge conflict: Pull request is not mergeable: the merge commit cannot be cleanly created.\n",
			wantCalls: []string{
				"AllowedMergeMethods",
				"ListOpenDependabotPRs",
				"Merge 1 rebase deleteBranch=true",
				"RequestRebase 1",
				"Merge 4 rebase deleteBranch=true",
			},
		},
		{
			name:       "reports a failed rebase request",
			args:       []string{"merge", "1"},
			mergeErr:   map[int]error{1: errConflict},
			rebaseErr:  map[int]error{1: errors.New("GraphQL: Resource not accessible by integration")},
			wantStdout: "Merging PR #1...\n",
			wantStderr: "Failed to merge PR #1: merge conflict: Pull request is not mergeable: the merge commit cannot be cleanly created.\n" +
				"Failed to request a rebase of PR #1: GraphQL: Resource not accessible by integration\n",
			wantCalls: []string{
				"AllowedMergeMethods",
				"ListOpenDependabotPRs",
				"Merge 1 merge deleteBranch=true",
				"RequestRebase 1",
			},
		},
		{
			name:       "does not ask for a rebase on unrelated failures",
			args:       []string{"merge", "4", "--method", "squash"},
			mergeErr:   map[int]error{4: errors.New("GraphQL: Required status check is expected.")},
			wantStdout: "Merging PR #4...\n",
			wantStderr: "Failed to merge PR #4: GraphQL: Required status check is expected.\n",
			wantCalls: []string{
				"AllowedMergeMethods",
				"ListOpenDependabotPRs",
				"Merge 4 squash deleteBranch=true",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(t)
			client.mergeErr = tt.mergeErr
			client.rebaseErr = tt.rebaseErr

			got := runCommand(t, tt.args...)

			assertOutput(t, got, tt.wantStdout, tt.wantStderr)
			client.assertCalls(t, tt.wantCalls)
		})
	}
}

func TestMergeCommandErrors(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		allowed   dependabot.AllowedMergeMethods
		wantErr   string
		wantCalls []string
	}{
		{
			name:    "rejects an unknown merge method before any call",
			args:    []string{"merge", "--method", "octopus"},
			allowed: dependabot.AllowedMergeMethods{Merge: true},
			wantErr: `invalid merge method "octopus": must be one of merge, rebase, squash`,
		},
		{
			name:      "refuses a merge method disabled on the repository before touching pull requests",
			args:      []string{"merge", "--method", "merge"},
			allowed:   dependabot.AllowedMergeMethods{Squash: true},
			wantErr:   `merge method "merge" is not allowed on this repository; allowed: squash`,
			wantCalls: []string{"AllowedMergeMethods"},
		},
		{
			name:    "rejects an invalid pull request number before any call",
			args:    []string{"merge", "0"},
			allowed: dependabot.AllowedMergeMethods{Merge: true},
			wantErr: "invalid pull request number: 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(t)
			client.allowed = tt.allowed

			got := runCommand(t, tt.args...)

			assertCommandError(t, got, tt.wantErr)
			client.assertCalls(t, tt.wantCalls)
		})
	}
}
