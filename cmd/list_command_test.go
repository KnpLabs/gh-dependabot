package cmd

import (
	"errors"
	"testing"
)

func TestListCommand(t *testing.T) {
	t.Run("prints one plain tab-separated row per pull request", func(t *testing.T) {
		client := newFakeClient(t)

		got := runCommand(t, "list")

		assertOutput(t, got,
			"1\tBump bubbles\t✓ passing\t✓ yes\n"+
				"3\tBump checkout\t● pending\t✗ conflict\n"+
				"4\tBump cobra\tnone\t✓ yes\n"+
				"5\tBump x/sys\t✗ failing\t✓ yes\n",
			"",
		)
		client.assertCalls(t, []string{"ListOpenDependabotPRs"})
	})

	t.Run("prints nothing when there are no pull requests", func(t *testing.T) {
		client := newFakeClient(t)
		client.prs = nil

		got := runCommand(t, "list")

		assertOutput(t, got, "", "")
		client.assertCalls(t, []string{"ListOpenDependabotPRs"})
	})

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

	t.Run("sanitizes title controls to preserve one plain row", func(t *testing.T) {
		client := newFakeClient(t)
		client.prs = client.prs[:1]
		client.prs[0].Title = "Bump\tfoo\n\x1b[31mred\x00"

		got := runCommand(t, "list")

		assertOutput(t, got, "1\tBump    foo�red�\t✓ passing\t✓ yes\n", "")
		client.assertCalls(t, []string{"ListOpenDependabotPRs"})
	})

	t.Run("rejects arguments", func(t *testing.T) {
		newFakeClient(t)

		got := runCommand(t, "list", "1")

		if got.err == nil {
			t.Fatal("expected an argument error")
		}
	})
}
