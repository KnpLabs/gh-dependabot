package cmd

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:          "view <number>",
	Short:        "Show a Dependabot pull request and its unified diff",
	Example:      `  gh dependabot view 42`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		numbers, err := parsePRNumbers(args)
		if err != nil {
			return err
		}
		number := numbers[0]
		client, err := newClient()
		if err != nil {
			return err
		}
		details, err := client.OpenDependabotPR(number)
		if err != nil {
			return err
		}
		diff, err := client.Diff(number)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Title: %s\n\n%s\n\nChecks: %s\n\n%s",
			diffText(ansi.Strip(details.PullRequest.Title)), details.Body, ansi.Strip(checksStatus(details.PullRequest.Checks, false)), diff)
		return err
	},
}

func init() {
	rootCmd.AddCommand(viewCmd)
}
