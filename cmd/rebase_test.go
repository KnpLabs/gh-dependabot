package cmd

import (
	"errors"
	"testing"
)

func TestRebaseCommand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		rebaseErr  map[int]error
		wantStdout string
		wantStderr string
		wantCalls  []string
	}{
		{
			name:       "asks to rebase every conflicting or failing pull request",
			args:       []string{"rebase"},
			wantStdout: "Requesting a rebase of PR #3...\nPR #3 rebase requested.\nRequesting a rebase of PR #5...\nPR #5 rebase requested.\n",
			wantCalls:  []string{"ListOpenDependabotPRs", "RequestRebase 3", "RequestRebase 5"},
		},
		{
			name:       "asks to rebase only the requested pull requests",
			args:       []string{"rebase", "5", "1"},
			wantStdout: "Requesting a rebase of PR #5...\nPR #5 rebase requested.\n",
			wantCalls:  []string{"ListOpenDependabotPRs", "RequestRebase 5"},
		},
		{
			name:       "skips pull requests that do not need a rebase",
			args:       []string{"rebase", "1", "4"},
			wantStdout: "No eligible dependabot pull requests found.\n",
			wantCalls:  []string{"ListOpenDependabotPRs"},
		},
		{
			name:       "reports a failed rebase request and carries on",
			args:       []string{"rebase"},
			rebaseErr:  map[int]error{3: errors.New("GraphQL: Resource not accessible by integration")},
			wantStdout: "Requesting a rebase of PR #3...\nRequesting a rebase of PR #5...\nPR #5 rebase requested.\n",
			wantStderr: "Failed to request a rebase of PR #3: GraphQL: Resource not accessible by integration\n",
			wantCalls:  []string{"ListOpenDependabotPRs", "RequestRebase 3", "RequestRebase 5"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(t)
			client.rebaseErr = tt.rebaseErr

			got := runCommand(t, tt.args...)

			assertOutput(t, got, tt.wantStdout, tt.wantStderr)
			client.assertCalls(t, tt.wantCalls)
		})
	}
}

func TestRebaseCommandErrors(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		listErr   error
		wantErr   string
		wantCalls []string
	}{
		{
			name:    "rejects an invalid pull request number before any call",
			args:    []string{"rebase", "3", "#5"},
			wantErr: "invalid pull request number: #5",
		},
		{
			name:      "returns the listing error",
			args:      []string{"rebase"},
			listErr:   errors.New("failed to query pull requests: boom"),
			wantErr:   "failed to query pull requests: boom",
			wantCalls: []string{"ListOpenDependabotPRs"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(t)
			client.listErr = tt.listErr

			got := runCommand(t, tt.args...)

			assertCommandError(t, got, tt.wantErr)
			client.assertCalls(t, tt.wantCalls)
		})
	}
}
