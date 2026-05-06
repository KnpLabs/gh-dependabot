package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/cli/go-gh/v2"
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
		methodFlag, err := mergeMethodFlag(method)
		if err != nil {
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

		prs, err := listDependabotPRs()
		if err != nil {
			return err
		}

		// Filter to target PRs if specified
		if len(targetPRs) > 0 {
			prs = filterByNumbers(prs, targetPRs)
		}

		if len(prs) == 0 {
			fmt.Println("No eligible dependabot pull requests found.")
			return nil
		}

		for _, pr := range prs {
			fmt.Printf("Merging PR #%d...\n", pr.Number)
			_, _, err := gh.Exec("pr", "merge", strconv.Itoa(pr.Number), methodFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to merge PR #%d: %v\n", pr.Number, err)
				continue
			}
			fmt.Printf("PR #%d merged.\n", pr.Number)
		}

		return nil
	},
}

func mergeMethodFlag(method string) (string, error) {
	switch method {
	case "merge":
		return "--merge", nil
	case "rebase":
		return "--rebase", nil
	case "squash":
		return "--squash", nil
	default:
		return "", fmt.Errorf("invalid merge method %q: must be one of merge, rebase, squash", method)
	}
}

func init() {
	mergeCmd.Flags().String("method", "merge", "Merge method to use: merge, rebase, or squash")
	rootCmd.AddCommand(mergeCmd)
}
