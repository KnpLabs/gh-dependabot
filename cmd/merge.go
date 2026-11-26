package cmd

import (
	"errors"
	"fmt"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/knplabs/gh-dependabot/internal/github"
	"github.com/spf13/cobra"
)

var mergeCmd = &cobra.Command{
	Use:   "merge [pull request numbers...]",
	Short: "Merge eligible Dependabot pull requests",
	Long: `Merge mergeable Dependabot pull requests with no status checks or with all checks
passing or skipped. Pass PR numbers to target specific PRs.
With no PR numbers, this command merges every eligible Dependabot PR in the current repository.

If a merge fails because of a conflict, this command requests a Dependabot rebase.

The merge method can be configured with the --method flag (merge, rebase, squash).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		methodFlag, err := cmd.Flags().GetString("method")
		if err != nil {
			return err
		}
		deleteBranch, err := cmd.Flags().GetBool("delete-branch")
		if err != nil {
			return err
		}

		numbers, err := parsePRNumbers(args)
		if err != nil {
			return err
		}

		client, err := newClient()
		if err != nil {
			return err
		}

		method, err := resolveMergeMethod(client, methodFlag)
		if err != nil {
			return err
		}

		return runOnEligiblePRs(cmd, client, numbers, bulkAction{
			selects:  dependabot.PullRequest.Eligible,
			verb:     "merge",
			progress: "Merging",
			done:     "merged",
			apply: func(number int) error {
				return client.Merge(number, method, deleteBranch)
			},
			onFailure: func(cmd *cobra.Command, number int, err error) {
				if !errors.Is(err, github.ErrConflict) {
					return
				}
				if err := client.RequestRebase(number); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to request a rebase of PR #%d: %v\n", number, err)
					return
				}
				fmt.Fprintf(cmd.OutOrStdout(), "PR #%d has conflicts, asked Dependabot to rebase it.\n", number)
			},
		})
	},
}

func init() {
	mergeCmd.Flags().String("method", "merge", "Merge method to use: merge, rebase, or squash")
	mergeCmd.Flags().Bool("delete-branch", true, "Delete the branch after merge")
	rootCmd.AddCommand(mergeCmd)
}
