package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/spf13/cobra"
)

var approveCmd = &cobra.Command{
	Use:   "approve [pull request numbers...]",
	Short: "Approve dependabot's pull requests that have passed or skipped status checks and are mergeable",
	Long: `Approve dependabot's pull requests that have either a status check
marked as passed or skipped and are mergeable. Optionally specify one or more
pull request numbers to target specific PRs, otherwise all matching PRs are targeted.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var targetPRs []int
		for _, arg := range args {
			num, err := strconv.Atoi(arg)
			if err != nil {
				return fmt.Errorf("invalid pull request number: %s", arg)
			}
			targetPRs = append(targetPRs, num)
		}

		client, err := newClient()
		if err != nil {
			return err
		}

		prs, err := fetchEligiblePullRequests(client)
		if err != nil {
			return err
		}

		if len(targetPRs) > 0 {
			prs = dependabot.FilterByNumbers(prs, targetPRs)
		}

		if len(prs) == 0 {
			fmt.Println("No eligible dependabot pull requests found.")
			return nil
		}

		for _, pr := range prs {
			fmt.Printf("Approving PR #%d...\n", pr.Number)
			if err := client.Approve(pr.Number); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to approve PR #%d: %v\n", pr.Number, err)
				continue
			}
			fmt.Printf("PR #%d approved.\n", pr.Number)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(approveCmd)
}
