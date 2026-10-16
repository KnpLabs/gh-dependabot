package cmd

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/spf13/cobra"
)

var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Interactively review Dependabot pull requests one by one",
	Long: `Iterate over every open Dependabot pull request, display its diff, and
prompt to approve (y) or skip (n) each one.

The default action shown in the footer flips based on CI status:
  - checks passing/pending: [Y/n] — enter approves
  - any check failing:      [y/N] — enter skips

Press r to post a "@dependabot rebase" comment on the current PR and move
on; this is independent of approve/merge and is available whatever the CI
status.

Skipping has no GitHub side effect: the PR is left untouched and can be
reviewed again on the next run. A summary lists approved and skipped PR
numbers when the loop ends or you quit early.

Pass --merge to also merge each PR after approval, using the method
specified by --method (merge, rebase, squash).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mergeAfterApprove, err := cmd.Flags().GetBool("merge")
		if err != nil {
			return err
		}
		methodFlag, err := cmd.Flags().GetString("method")
		if err != nil {
			return err
		}
		deleteBranch, err := cmd.Flags().GetBool("delete-branch")
		if err != nil {
			return err
		}

		client, err := newClient()
		if err != nil {
			return err
		}

		var mergeMethod dependabot.MergeMethod
		if mergeAfterApprove {
			mergeMethod, err = resolveMergeMethod(client, methodFlag)
			if err != nil {
				return err
			}
		}

		m := newInteractiveModel(client)
		m.mergeAfterApprove = mergeAfterApprove
		m.mergeMethod = mergeMethod
		m.deleteBranch = deleteBranch

		p := tea.NewProgram(m)
		final, err := p.Run()
		if err != nil {
			return fmt.Errorf("failed to run TUI: %w", err)
		}
		if im, ok := final.(interactiveModel); ok {
			im.printSummary(cmd.OutOrStdout())
		}
		return nil
	},
}

func init() {
	interactiveCmd.Flags().Bool("merge", false, "Merge each PR after approving it")
	interactiveCmd.Flags().String("method", "merge", "Merge method to use when --merge is set: merge, rebase, or squash")
	interactiveCmd.Flags().Bool("delete-branch", true, "Delete the branch after merge (only with --merge)")
	rootCmd.AddCommand(interactiveCmd)
}
