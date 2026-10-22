package cmd

import (
	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/spf13/cobra"
)

var rebaseCmd = &cobra.Command{
	Use:   "rebase [pull request numbers...]",
	Short: "Ask Dependabot to rebase its pull requests that have conflicts or failing status checks",
	Long: `Ask Dependabot to rebase its pull requests that either conflict with
their base branch or have a failing status check, by commenting "@dependabot rebase".
Optionally specify one or more pull request numbers to target specific PRs,
otherwise all matching PRs are targeted.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		numbers, err := parsePRNumbers(args)
		if err != nil {
			return err
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		return runOnEligiblePRs(cmd, client, numbers, bulkAction{
			selects:  dependabot.PullRequest.NeedsRebase,
			verb:     "request a rebase of",
			progress: "Requesting a rebase of",
			done:     "rebase requested",
			apply:    client.RequestRebase,
		})
	},
}

func init() {
	rootCmd.AddCommand(rebaseCmd)
}
