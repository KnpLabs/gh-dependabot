package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
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
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	repo, err := repository.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to determine current repository: %w", err)
	}

	query := `query DependabotPRsForApproval($owner: String!, $name: String!) {
		repository(owner: $owner, name: $name) {
			pullRequests(states: OPEN, first: 100) {
				nodes {
					number
					headRefName
					author { login }
					mergeable
					commits(last: 1) {
						nodes {
							commit {
								statusCheckRollup {
									contexts(first: 100) {
										nodes {
											... on CheckRun { conclusion }
											... on StatusContext { state }
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"owner": repo.Owner,
		"name":  repo.Name,
	}

	var result struct {
		Repository struct {
			PullRequests struct {
				Nodes []struct {
					Number      int `json:"number"`
					HeadRefName string `json:"headRefName"`
					Author      struct {
						Login string `json:"login"`
					} `json:"author"`
					Mergeable string `json:"mergeable"`
					Commits   struct {
						Nodes []struct {
							Commit struct {
								StatusCheckRollup *struct {
									Contexts struct {
										Nodes []struct {
											Conclusion string `json:"conclusion"`
											State      string `json:"state"`
										} `json:"nodes"`
									} `json:"contexts"`
								} `json:"statusCheckRollup"`
							} `json:"commit"`
						} `json:"nodes"`
					} `json:"commits"`
				} `json:"nodes"`
			} `json:"pullRequests"`
		} `json:"repository"`
	}

	if err := client.Do(query, variables, &result); err != nil {
		return nil, fmt.Errorf("failed to query pull requests: %w", err)
	}

	var eligible []pullRequest
	for _, node := range result.Repository.PullRequests.Nodes {
		if !isDependabotPR(node.Author.Login, node.HeadRefName) {
			continue
		}

		var checks []statusCheck
		if len(node.Commits.Nodes) > 0 && node.Commits.Nodes[0].Commit.StatusCheckRollup != nil {
			for _, ctx := range node.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes {
				conclusion := ctx.Conclusion
				if conclusion == "" {
					conclusion = mapStateToConclusion(ctx.State)
				}
				checks = append(checks, statusCheck{Conclusion: conclusion})
			}
		}

		if !hasPassingChecks(checks) {
			continue
		}
		if node.Mergeable != "MERGEABLE" {
			continue
		}

		eligible = append(eligible, pullRequest{
			Number:            node.Number,
			Author:            author{Login: node.Author.Login},
			StatusCheckRollup: checks,
			Mergeable:         node.Mergeable,
		})
	}

	return eligible, nil
}

func isDependabotPR(login string, headRefName string) bool {
	return isDependabotAuthor(login) || strings.HasPrefix(headRefName, "dependabot/")
}

func isDependabotAuthor(login string) bool {
	return login == "app/dependabot" || login == "dependabot[bot]" || login == "dependabot"
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
