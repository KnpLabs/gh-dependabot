package cmd

import (
	"github.com/spf13/cobra"
)

var approveCmd = &cobra.Command{
	Use:   "approve [pull request numbers...]",
	Short: "Approve dependabot's pull requests that have passed or skipped status checks and are mergeable",
	Long: `Approve dependabot's pull requests that have either a status check
marked as passed or skipped and are mergeable. Optionally specify one or more
pull request numbers to target specific PRs, otherwise all matching PRs are targeted.`,
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
