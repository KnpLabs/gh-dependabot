package github

import (
	"fmt"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

const dependabotPRFields = `fragment DependabotPRFields on PullRequest {
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
}`

const dependabotPRsQuery = `query DependabotPRs($owner: String!, $name: String!, $after: String) {
		repository(owner: $owner, name: $name) {
			pullRequests(states: OPEN, first: 100, after: $after) {
				pageInfo { hasNextPage endCursor }
				nodes {
					...DependabotPRFields
				}
			}
		}
	}` + "\n" + dependabotPRFields

const dependabotPRByNumberQuery = `query DependabotPRByNumber($owner: String!, $name: String!, $number: Int!) {
	repository(owner: $owner, name: $name) {
		pullRequest(number: $number) {
			body
			state
			...DependabotPRFields
		}
	}
}` + "\n" + dependabotPRFields

type pullRequestNode struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	State       string `json:"state"`
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

func (c *client) OpenDependabotPR(number int) (PullRequestDetails, error) {
	variables := c.repoVariables()
	variables["number"] = number
	var result struct {
		Repository struct {
			PullRequest *pullRequestNode `json:"pullRequest"`
		} `json:"repository"`
	}
	if err := c.graphql.Do(dependabotPRByNumberQuery, variables, &result); err != nil {
		return PullRequestDetails{}, fmt.Errorf("failed to query pull request #%d: %w", number, err)
	}
	node := result.Repository.PullRequest
	if node == nil {
		return PullRequestDetails{}, fmt.Errorf("pull request #%d not found", number)
	}
	if node.State != "OPEN" {
		return PullRequestDetails{}, fmt.Errorf("pull request #%d is not open", number)
	}
	pr := node.toPullRequest()
	if !pr.IsDependabot() {
		return PullRequestDetails{}, fmt.Errorf("pull request #%d is not a Dependabot pull request", number)
	}
	return PullRequestDetails{PullRequest: pr, Body: node.Body}, nil
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
