package cmd

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:          "view <number>",
	Short:        "Show a Dependabot pull request, its unified diff, checks, and labels",
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
		labels := make([]string, 0, len(details.Labels))
		for _, label := range details.Labels {
			labels = append(labels, plainPRTitle(label))
		}
		labelText := "none"
		if len(labels) > 0 {
			labelText = strings.Join(labels, ", ")
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Title: %s\n\n%s\n\nChecks: %s\nMergeable: %s\nLabels: %s\n\n%s",
			plainPRTitle(details.PullRequest.Title), plainReviewText(details.Body),
			checkLabel(details.PullRequest.Checks).text, mergeabilityLabel(details.PullRequest.Mergeable).text, labelText, plainReviewText(diff))
		return err
	},
}

func plainReviewText(text string) string {
	text = strings.ReplaceAll(ansi.Strip(text), "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return '�'
		}
		return r
	}, text)
}

func init() {
	rootCmd.AddCommand(viewCmd)
}
