package cmd

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/knplabs/gh-dependabot/internal/github"
	"github.com/spf13/cobra"
)

var newClient = github.Default

var rootCmd = &cobra.Command{
	Use:   "gh-dependabot",
	Short: "A GitHub CLI extension to manage Dependabot pull requests",
	Long: `A GitHub CLI extension to interact with pull requests opened by Dependabot on GitHub.

When called without any subcommand, it displays an interactive table of all
open pull requests authored by Dependabot, showing:
  - PR number
  - Title
  - Status check result
  - Mergeable status

Use the arrow keys to navigate and q to quit.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		m := newListModel(client)
		p := tea.NewProgram(m)
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("failed to run TUI: %w", err)
		}
		return nil
	},
}

func fetchEligiblePullRequests(client github.Client) ([]dependabot.PullRequest, error) {
	prs, err := client.ListOpenDependabotPRs()
	if err != nil {
		return nil, err
	}
	var eligible []dependabot.PullRequest
	for _, pr := range prs {
		if pr.Eligible() {
			eligible = append(eligible, pr)
		}
	}
	return eligible, nil
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
