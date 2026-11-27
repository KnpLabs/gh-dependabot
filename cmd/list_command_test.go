package cmd

import (
	"errors"
	"testing"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

func TestListCommand(t *testing.T) {
	tests := []struct {
		name string
		prs  []dependabot.PullRequest
		want string
	}{
		{
			name: "prints one plain tab-separated row per pull request",
			prs:  openPullRequests,
			want: "1\tBump bubbles\t✓ passing\t✓ yes\n" +
				"3\tBump checkout\t● pending\t✗ conflict\n" +
				"4\tBump cobra\tnone\t✓ yes\n" +
				"5\tBump x/sys\t✗ failing\t✓ yes\n",
		},
		{
			name: "prints nothing when there are no pull requests",
		},
		{
			name: "sanitizes title controls to preserve one plain row",
			prs: []dependabot.PullRequest{
				{Number: 1, Title: "Bump\tfoo\n\x1b[31mred\x00", Checks: dependabot.CheckPassing, Mergeable: dependabot.Mergeable},
			},
			want: "1\tBump    foo�red�\t✓ passing\t✓ yes\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(t)
			client.prs = tt.prs

			got := runCommand(t, "list")

			assertOutput(t, got, tt.want, "")
			client.assertCalls(t, []string{"ListOpenDependabotPRs"})
		})
	}

	t.Run("returns a listing error without output", func(t *testing.T) {
		client := newFakeClient(t)
		client.listErr = errors.New("failed to query pull requests: boom")

		got := runCommand(t, "list")

		assertCommandError(t, got, "failed to query pull requests: boom")
		if got.stdout != "" {
			t.Errorf("stdout = %q, want empty", got.stdout)
		}
		if got.stderr != "Error: failed to query pull requests: boom\n" {
			t.Errorf("stderr = %q, want error message", got.stderr)
		}
		client.assertCalls(t, []string{"ListOpenDependabotPRs"})
	})

	t.Run("rejects arguments", func(t *testing.T) {
		client := newFakeClient(t)

		got := runCommand(t, "list", "1")

		if got.err == nil {
			t.Fatal("expected an argument error")
		}
		client.assertCalls(t, nil)
	})
}
