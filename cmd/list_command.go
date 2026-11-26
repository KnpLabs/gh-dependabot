package cmd

import (
	"fmt"

	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:          "list",
	Short:        "List open Dependabot pull requests as plain text",
	Example:      `  gh dependabot list`,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}

		prs, err := client.ListOpenDependabotPRs()
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		for _, pr := range prs {
			title := plainPRTitle(pr.Title)
			if _, err := fmt.Fprintf(out, "%d\t%s\t%s\t%s\n",
				pr.Number, title, checkLabel(pr.Checks).text, mergeabilityLabel(pr.Mergeable).text); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func plainPRTitle(title string) string {
	return diffText(ansi.Strip(title))
}
