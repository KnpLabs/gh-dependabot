package github

import (
	"fmt"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

const dependabotPRsQuery = `query DependabotPRs($owner: String!, $name: String!, $after: String) {
		repository(owner: $owner, name: $name) {
			pullRequests(states: OPEN, first: 100, after: $after) {
				pageInfo { hasNextPage endCursor }
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

func (c *client) ListOpenDependabotPRs() ([]dependabot.PullRequest, error) {
	var prs []dependabot.PullRequest
	variables := c.repoVariables()
	seenCursors := map[string]bool{}
	for {
		var result struct {
			Repository struct {
				PullRequests struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []pullRequestNode `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		}

		if err := c.graphql.Do(dependabotPRsQuery, variables, &result); err != nil {
			return nil, fmt.Errorf("failed to query pull requests: %w", err)
		}

		for _, node := range result.Repository.PullRequests.Nodes {
			pr := node.toPullRequest()
			if pr.IsDependabot() {
				prs = append(prs, pr)
			}
		}
		if !result.Repository.PullRequests.PageInfo.HasNextPage {
			return prs, nil
		}
		if result.Repository.PullRequests.PageInfo.EndCursor == "" {
			return nil, fmt.Errorf("failed to query pull requests: next page has no end cursor")
		}
		cursor := result.Repository.PullRequests.PageInfo.EndCursor
		if seenCursors[cursor] {
			return nil, fmt.Errorf("failed to query pull requests: repeated end cursor %q", cursor)
		}
		seenCursors[cursor] = true
		variables["after"] = cursor
	}
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
