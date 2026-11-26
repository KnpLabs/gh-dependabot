package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/knplabs/gh-dependabot/internal/github"
)

func TestViewCommand(t *testing.T) {
	t.Run("prints the title, full body, check status, and unified diff", func(t *testing.T) {
		client := newFakeClient(t)
		client.view = github.PullRequestDetails{
			PullRequest: dependabot.PullRequest{Number: 41, Title: "Bump example", Checks: dependabot.CheckPassing},
			Body:        "Full body line one.\n\nFull body line two.",
		}
		client.diffs = map[int]string{41: "diff --git a/go.mod b/go.mod\n+go 1.2\n"}

		got := runCommand(t, "view", "41")

		assertOutput(t, got,
			"Title: Bump example\n\nFull body line one.\n\nFull body line two.\n\nChecks: ✓ passing\n\n"+
				"diff --git a/go.mod b/go.mod\n+go 1.2\n",
			"",
		)
		client.assertCalls(t, []string{"OpenDependabotPR 41", "Diff 41"})
	})

	tests := []struct {
		name      string
		args      []string
		viewErr   error
		diffErr   error
		wantErr   string
		wantCalls []string
	}{
		{name: "rejects an invalid pull request number", args: []string{"view", "abc"}, wantErr: "invalid pull request number: abc"},
		{name: "rejects a non-positive pull request number", args: []string{"view", "0"}, wantErr: "invalid pull request number: 0"},
		{name: "returns an out-of-scope error without output", args: []string{"view", "41"}, viewErr: errors.New("pull request #41 is not a Dependabot pull request"), wantErr: "pull request #41 is not a Dependabot pull request", wantCalls: []string{"OpenDependabotPR 41"}},
		{name: "returns a client error without output", args: []string{"view", "41"}, viewErr: errors.New("failed to query pull request #41: boom"), wantErr: "failed to query pull request #41: boom", wantCalls: []string{"OpenDependabotPR 41"}},
		{name: "returns a diff error without partial output", args: []string{"view", "41"}, diffErr: errors.New("failed to get diff: boom"), wantErr: "failed to get diff: boom", wantCalls: []string{"OpenDependabotPR 41", "Diff 41"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(t)
			client.viewErr = tt.viewErr
			if tt.diffErr != nil {
				client.diffErr = map[int]error{41: tt.diffErr}
			}

			got := runCommand(t, tt.args...)

			assertCommandError(t, got, tt.wantErr)
			if got.stdout != "" {
				t.Errorf("stdout = %q, want empty", got.stdout)
			}
			client.assertCalls(t, tt.wantCalls)
		})
	}

	t.Run("root help lists view", func(t *testing.T) {
		newFakeClient(t)

		got := runCommand(t, "--help")

		if got.err != nil {
			t.Fatalf("help error: %v", got.err)
		}
		if !strings.Contains(got.stdout, "view") {
			t.Errorf("root help does not list view: %q", got.stdout)
		}
	})
}
