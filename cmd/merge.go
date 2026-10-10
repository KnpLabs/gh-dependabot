package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/knplabs/gh-dependabot/internal/github"
	"github.com/spf13/cobra"
)

var mergeCmd = &cobra.Command{
	Use:   "merge [pull request numbers...]",
	Short: "Merge dependabot's pull requests that have passed or skipped status checks and are mergeable",
	Long: `Merge dependabot's pull requests that have either a status check
marked as passed or skipped and are mergeable. Optionally specify one or more
pull request numbers to target specific PRs, otherwise all matching PRs are targeted.

The merge method can be configured with the --method flag (merge, rebase, squash).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		method, err := cmd.Flags().GetString("method")
		if err != nil {
			return err
		}
		mergeMethod, err := dependabot.ParseMergeMethod(method)
		if err != nil {
			return err
		}

		client, err := newClient()
		if err != nil {
			return err
		}

		allowed, err := client.AllowedMergeMethods()
		if err != nil {
			return err
		}
		if err := allowed.Validate(mergeMethod); err != nil {
			return err
		}

		var targetPRs []int
		for _, arg := range args {
			num, err := strconv.Atoi(arg)
			if err != nil {
				return fmt.Errorf("invalid pull request number: %s", arg)
			}
			targetPRs = append(targetPRs, num)
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

		deleteBranch, err := cmd.Flags().GetBool("delete-branch")
		if err != nil {
			return err
		}

		for _, pr := range prs {
			fmt.Printf("Merging PR #%d...\n", pr.Number)
			if err := client.Merge(pr.Number, mergeMethod, deleteBranch); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to merge PR #%d: %v\n", pr.Number, err)
				if errors.Is(err, github.ErrConflict) {
					if err := client.RequestRebase(pr.Number); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to request a rebase of PR #%d: %v\n", pr.Number, err)
					} else {
						fmt.Printf("PR #%d has conflicts, asked Dependabot to rebase it.\n", pr.Number)
					}
				}
				continue
			}
			fmt.Printf("PR #%d merged.\n", pr.Number)
		}

		return nil
	},
}

func init() {
	mergeCmd.Flags().String("method", "merge", "Merge method to use: merge, rebase, or squash")
	mergeCmd.Flags().Bool("delete-branch", true, "Delete the branch after merge")
	rootCmd.AddCommand(mergeCmd)
}
