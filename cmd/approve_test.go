package cmd

import (
	"errors"
	"testing"
)

func TestApproveCommand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		approveErr map[int]error
		wantStdout string
		wantStderr string
		wantCalls  []string
	}{
		{
			name:       "approves every eligible pull request",
			args:       []string{"approve"},
			wantStdout: "Approving PR #1...\nPR #1 approved.\nApproving PR #4...\nPR #4 approved.\n",
			wantCalls:  []string{"ListOpenDependabotPRs", "Approve 1", "Approve 4"},
		},
		{
			name:       "approves only the requested pull requests",
			args:       []string{"approve", "4", "5"},
			wantStdout: "Approving PR #4...\nPR #4 approved.\n",
			wantCalls:  []string{"ListOpenDependabotPRs", "Approve 4"},
		},
		{
			name:       "approves a pull request requested twice once",
			args:       []string{"approve", "4", "4"},
			wantStdout: "Approving PR #4...\nPR #4 approved.\n",
			wantCalls:  []string{"ListOpenDependabotPRs", "Approve 4"},
		},
		{
			name:       "skips pull requests that are not eligible",
			args:       []string{"approve", "3", "5"},
			wantStdout: "No eligible dependabot pull requests found.\n",
			wantCalls:  []string{"ListOpenDependabotPRs"},
		},
		{
			name:       "reports a failed approval and carries on",
			args:       []string{"approve"},
			approveErr: map[int]error{1: errors.New("GraphQL: Review cannot be requested from pull request author.")},
			wantStdout: "Approving PR #1...\nApproving PR #4...\nPR #4 approved.\n",
			wantStderr: "Failed to approve PR #1: GraphQL: Review cannot be requested from pull request author.\n",
			wantCalls:  []string{"ListOpenDependabotPRs", "Approve 1", "Approve 4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(t)
			client.approveErr = tt.approveErr

			got := runCommand(t, tt.args...)

			assertOutput(t, got, tt.wantStdout, tt.wantStderr)
			client.assertCalls(t, tt.wantCalls)
		})
	}
}

func TestApproveCommandErrors(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		listErr   error
		wantErr   string
		wantCalls []string
	}{
		{
			name:    "rejects an invalid pull request number before any call",
			args:    []string{"approve", "4", "abc"},
			wantErr: "invalid pull request number: abc",
		},
		{
			name:      "returns the listing error",
			args:      []string{"approve"},
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
