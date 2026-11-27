package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	gh "github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "update",
		Short: "Update the extension and its installed agent skill",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return applyUpdates(cmd, cmd.OutOrStdout())
		},
	})
}

func checkForUpdates(cmd *cobra.Command, _ []string) {
	if cmd.Hidden || cmd.Name() == "update" || cmd.Name() == "completion" ||
		os.Getenv("GH_EXTENSION") != "1" || os.Getenv("GH_NO_EXTENSION_UPDATE_NOTIFIER") != "" {
		return
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Second)
	defer cancel()
	if !updateAvailable(ctx) {
		return
	}

	output := cmd.ErrOrStderr()
	if term.IsTerminal(os.Stdin) && term.IsTerminal(os.Stderr) {
		fmt.Fprint(output, "Update gh-dependabot and its installed agent skill now? [y/N] ")
		var answer string
		if _, err := fmt.Fscanln(cmd.InOrStdin(), &answer); err != nil || !strings.EqualFold(answer, "y") {
			return
		}
	}
	if err := applyUpdates(cmd, output); err != nil {
		fmt.Fprintln(output, err)
	}
}

func updateAvailable(ctx context.Context) bool {
	output, _, err := gh.ExecContext(ctx, "extension", "upgrade", "dependabot", "--dry-run")
	if err == nil && strings.Contains(output.String(), "[dependabot]: would have upgraded from ") {
		return true
	}
	output, _, err = gh.ExecContext(ctx, "skill", "update", "gh-dependabot", "--all", "--dry-run")
	return err == nil && strings.TrimSpace(output.String()) != ""
}

func applyUpdates(cmd *cobra.Command, output io.Writer) error {
	if err := runUpdate(cmd, output, "extension", "upgrade", "dependabot"); err != nil {
		return err
	}
	return runUpdate(cmd, output, "skill", "update", "gh-dependabot", "--all")
}

func runUpdate(cmd *cobra.Command, output io.Writer, args ...string) error {
	stdout, stderr, err := gh.ExecContext(cmd.Context(), args...)
	fmt.Fprint(output, stdout.String())
	fmt.Fprint(cmd.ErrOrStderr(), stderr.String())
	if err != nil {
		return fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
