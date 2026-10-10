package cmd

import (
	"fmt"
	"strconv"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/knplabs/gh-dependabot/internal/github"
	"github.com/spf13/cobra"
)

type bulkAction struct {
	verb      string
	progress  string
	done      string
	apply     func(number int) error
	onFailure func(cmd *cobra.Command, number int, err error)
}

func runOnEligiblePRs(cmd *cobra.Command, client github.Client, numbers []int, action bulkAction) error {
	prs, err := fetchEligiblePullRequests(client)
	if err != nil {
		return err
	}

	if len(numbers) > 0 {
		prs = dependabot.FilterByNumbers(prs, numbers)
	}

	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
	if len(prs) == 0 {
		fmt.Fprintln(out, "No eligible dependabot pull requests found.")
		return nil
	}

	for _, pr := range prs {
		fmt.Fprintf(out, "%s PR #%d...\n", action.progress, pr.Number)
		if err := action.apply(pr.Number); err != nil {
			fmt.Fprintf(errOut, "Failed to %s PR #%d: %v\n", action.verb, pr.Number, err)
			if action.onFailure != nil {
				action.onFailure(cmd, pr.Number, err)
			}
			continue
		}
		fmt.Fprintf(out, "PR #%d %s.\n", pr.Number, action.done)
	}

	return nil
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

func parsePRNumbers(args []string) ([]int, error) {
	var numbers []int
	seen := make(map[int]bool)
	for _, arg := range args {
		number, err := strconv.Atoi(arg)
		if err != nil || number <= 0 {
			return nil, fmt.Errorf("invalid pull request number: %s", arg)
		}
		if seen[number] {
			continue
		}
		seen[number] = true
		numbers = append(numbers, number)
	}
	return numbers, nil
}

func resolveMergeMethod(client github.Client, flag string) (dependabot.MergeMethod, error) {
	method, err := dependabot.ParseMergeMethod(flag)
	if err != nil {
		return "", err
	}
	allowed, err := client.AllowedMergeMethods()
	if err != nil {
		return "", err
	}
	if err := allowed.Validate(method); err != nil {
		return "", err
	}
	return method, nil
}
