package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"
)

type allowedMergeMethods struct {
	Merge  bool
	Squash bool
	Rebase bool
}

func fetchAllowedMergeMethods() (allowedMergeMethods, error) {
	client, err := newGraphQLClient()
	if err != nil {
		return allowedMergeMethods{}, fmt.Errorf("failed to create GraphQL client: %w", err)
	}
	repo, err := repository.Current()
	if err != nil {
		return allowedMergeMethods{}, fmt.Errorf("failed to determine current repository: %w", err)
	}

	query := `query RepoMergeMethods($owner: String!, $name: String!) {
		repository(owner: $owner, name: $name) {
			mergeCommitAllowed
			squashMergeAllowed
			rebaseMergeAllowed
		}
	}`
	variables := map[string]interface{}{
		"owner": repo.Owner,
		"name":  repo.Name,
	}

	var result struct {
		Repository struct {
			MergeCommitAllowed bool `json:"mergeCommitAllowed"`
			SquashMergeAllowed bool `json:"squashMergeAllowed"`
			RebaseMergeAllowed bool `json:"rebaseMergeAllowed"`
		} `json:"repository"`
	}
	if err := client.Do(query, variables, &result); err != nil {
		return allowedMergeMethods{}, fmt.Errorf("failed to query repository merge settings: %w", err)
	}
	return allowedMergeMethods{
		Merge:  result.Repository.MergeCommitAllowed,
		Squash: result.Repository.SquashMergeAllowed,
		Rebase: result.Repository.RebaseMergeAllowed,
	}, nil
}

func validateMergeMethod(method string, allowed allowedMergeMethods) error {
	var ok bool
	switch method {
	case "merge":
		ok = allowed.Merge
	case "squash":
		ok = allowed.Squash
	case "rebase":
		ok = allowed.Rebase
	default:
		return fmt.Errorf("invalid merge method %q: must be one of merge, rebase, squash", method)
	}
	if ok {
		return nil
	}

	var enabled []string
	if allowed.Merge {
		enabled = append(enabled, "merge")
	}
	if allowed.Squash {
		enabled = append(enabled, "squash")
	}
	if allowed.Rebase {
		enabled = append(enabled, "rebase")
	}
	if len(enabled) == 0 {
		return fmt.Errorf("merge method %q is not allowed on this repository (no merge methods are enabled)", method)
	}
	return fmt.Errorf("merge method %q is not allowed on this repository; allowed: %s", method, strings.Join(enabled, ", "))
}

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

		allowed, err := fetchAllowedMergeMethods()
		if err != nil {
			return err
		}
		if err := validateMergeMethod(method, allowed); err != nil {
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

		if len(targetPRs) > 0 {
			prs = filterByNumbers(prs, targetPRs)
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
			args := []string{"pr", "merge", strconv.Itoa(pr.Number), methodFlag}
			if deleteBranch {
				args = append(args, "--delete-branch")
			}
			_, stderr, err := ghExec(args...)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to merge PR #%d: %v\n", pr.Number, err)
				if stderr.Len() > 0 {
					fmt.Fprintf(os.Stderr, "%s\n", stderr.String())
				}
				if isMergeConflict(pr.Number, stderr.String()) {
					if err := requestRebase(pr.Number); err != nil {
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

func isMergeConflict(number int, stderr string) bool {
	if strings.Contains(stderr, "cannot be cleanly created") {
		return true
	}
	stdout, _, err := ghExec("pr", "view", strconv.Itoa(number), "--json", "mergeable", "--jq", ".mergeable")
	return err == nil && strings.TrimSpace(stdout.String()) == "CONFLICTING"
}

func requestRebase(number int) error {
	_, stderr, err := ghExec("pr", "comment", strconv.Itoa(number), "--body", "@dependabot rebase")
	if err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return err
	}
	return nil
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
	mergeCmd.Flags().Bool("delete-branch", true, "Delete the branch after merge")
	rootCmd.AddCommand(mergeCmd)
}
