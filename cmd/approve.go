package cmd

import (
	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/spf13/cobra"
)

var approveCmd = &cobra.Command{
	Use:   "approve [pull request numbers...]",
	Short: "Approve eligible Dependabot pull requests",
	Long: `Approve mergeable Dependabot pull requests with no status checks or with all checks
passing or skipped. Pass PR numbers to target specific PRs.
With no PR numbers, this command approves every eligible Dependabot PR in the current repository.`,
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
			selects:  dependabot.PullRequest.Eligible,
			verb:     "approve",
			progress: "Approving",
			done:     "approved",
			apply:    client.Approve,
		})
	},
}

func init() {
	rootCmd.AddCommand(approveCmd)
}
