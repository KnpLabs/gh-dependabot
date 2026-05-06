package cmd

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
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
		m := newListModel()
		p := tea.NewProgram(m)
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("failed to run TUI: %w", err)
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

