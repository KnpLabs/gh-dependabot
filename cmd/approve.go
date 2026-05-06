package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/cli/go-gh/v2"
	"github.com/spf13/cobra"
)

type statusCheck struct {
	Conclusion string `json:"conclusion"`
}

type author struct {
	Login string `json:"login"`
}

type pullRequest struct {
	Number             int           `json:"number"`
	Author             author        `json:"author"`
	StatusCheckRollup  []statusCheck `json:"statusCheckRollup"`
	Mergeable          string        `json:"mergeable"`
}

var approveCmd = &cobra.Command{
	Use:   "approve [pull request numbers...]",
	Short: "Approve dependabot's pull requests that have passed or skipped status checks and are mergeable",
	Long: `Approve dependabot's pull requests that have either a status check
marked as passed or skipped and are mergeable. Optionally specify one or more
pull request numbers to target specific PRs, otherwise all matching PRs are targeted.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
			fmt.Printf("Approving PR #%d...\n", pr.Number)
			_, _, err := gh.Exec("pr", "review", strconv.Itoa(pr.Number), "--approve")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to approve PR #%d: %v\n", pr.Number, err)
				continue
			}
			fmt.Printf("PR #%d approved.\n", pr.Number)
		}

		return nil
	},
}

func listDependabotPRs() ([]pullRequest, error) {
	stdout, _, err := gh.Exec("pr", "list", "--json", "number,author,statusCheckRollup,mergeable")
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests: %w", err)
	}

	var prs []pullRequest
	if err := json.Unmarshal(stdout.Bytes(), &prs); err != nil {
		return nil, fmt.Errorf("failed to parse pull requests: %w", err)
	}

	var eligible []pullRequest
	for _, pr := range prs {
		if !isDependabotAuthor(pr.Author.Login) {
			continue
		}
		if !hasPassingChecks(pr.StatusCheckRollup) {
			continue
		}
		if pr.Mergeable != "MERGEABLE" {
			continue
		}
		eligible = append(eligible, pr)
	}

	return eligible, nil
}

func isDependabotAuthor(login string) bool {
	return login == "app/dependabot" || login == "dependabot[bot]"
}

func hasPassingChecks(checks []statusCheck) bool {
	// No checks is acceptable (matches the jq logic where null is allowed)
	if len(checks) == 0 {
		return true
	}
	for _, check := range checks {
		if check.Conclusion != "SUCCESS" && check.Conclusion != "SKIPPED" {
			return false
		}
	}
	return true
}

func filterByNumbers(prs []pullRequest, numbers []int) []pullRequest {
	numSet := make(map[int]bool)
	for _, n := range numbers {
		numSet[n] = true
	}

	var filtered []pullRequest
	for _, pr := range prs {
		if numSet[pr.Number] {
			filtered = append(filtered, pr)
		}
	}
	return filtered
}

func init() {
	rootCmd.AddCommand(approveCmd)
}
