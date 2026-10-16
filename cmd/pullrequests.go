package cmd

import (
	"fmt"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

const dependabotPRsQuery = `query DependabotPRs($owner: String!, $name: String!) {
		repository(owner: $owner, name: $name) {
			pullRequests(states: OPEN, first: 100) {
				nodes {
					number
					title
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

type pullRequestNode struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
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
}

func fetchDependabotPullRequests() ([]dependabot.PullRequest, error) {
	client, err := newGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	repo, err := repository.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to determine current repository: %w", err)
	}

	variables := map[string]interface{}{
		"owner": repo.Owner,
		"name":  repo.Name,
	}

	var result struct {
		Repository struct {
			PullRequests struct {
				Nodes []pullRequestNode `json:"nodes"`
			} `json:"pullRequests"`
		} `json:"repository"`
	}

	if err := client.Do(dependabotPRsQuery, variables, &result); err != nil {
		return nil, fmt.Errorf("failed to query pull requests: %w", err)
	}

	var prs []dependabot.PullRequest
	for _, node := range result.Repository.PullRequests.Nodes {
		pr := node.toPullRequest()
		if pr.IsDependabot() {
			prs = append(prs, pr)
		}
	}
	return prs, nil
}

func fetchEligiblePullRequests() ([]dependabot.PullRequest, error) {
	prs, err := fetchDependabotPullRequests()
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

func (node pullRequestNode) toPullRequest() dependabot.PullRequest {
	var conclusions []string
	if len(node.Commits.Nodes) > 0 && node.Commits.Nodes[0].Commit.StatusCheckRollup != nil {
		for _, ctx := range node.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes {
			conclusion := ctx.Conclusion
			if conclusion == "" {
				conclusion = mapStateToConclusion(ctx.State)
			}
			conclusions = append(conclusions, conclusion)
		}
	}

	return dependabot.PullRequest{
		Number:      node.Number,
		Title:       node.Title,
		Author:      node.Author.Login,
		HeadRefName: node.HeadRefName,
		Checks:      dependabot.CheckStateOf(conclusions),
		Mergeable:   dependabot.Mergeability(node.Mergeable),
	}
}

func mapStateToConclusion(state string) string {
	switch state {
	case "SUCCESS":
		return "SUCCESS"
	case "PENDING", "EXPECTED":
		return "PENDING"
	case "FAILURE", "ERROR":
		return "FAILURE"
	default:
		return state
	}
}
