package github

import (
	"fmt"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

const checkContextFields = `fragment CheckContextFields on StatusCheckRollupContextConnection {
	pageInfo { hasNextPage endCursor }
	nodes {
		... on CheckRun { conclusion }
		... on StatusContext { state }
	}
}`

const checkContextsQuery = `query DependabotCheckContexts($commit: ID!, $after: String!) {
	node(id: $commit) {
		... on Commit {
			statusCheckRollup {
				contexts(first: 100, after: $after) { ...CheckContextFields }
			}
		}
	}
}` + "\n" + checkContextFields

const dependabotPRFields = `fragment DependabotPRFields on PullRequest {
	number
	title
	headRefName
	author { login }
	mergeable
	commits(last: 1) {
		nodes {
			commit {
				id
				statusCheckRollup {
					contexts(first: 100) { ...CheckContextFields }
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
	}` + "\n" + dependabotPRFields + "\n" + checkContextFields

const dependabotPRByNumberQuery = `query DependabotPRByNumber($owner: String!, $name: String!, $number: Int!) {
	repository(owner: $owner, name: $name) {
		pullRequest(number: $number) {
			bodyText
			state
			...DependabotPRFields
		}
	}
}` + "\n" + dependabotPRFields + "\n" + checkContextFields

type pullRequestNode struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"bodyText"`
	State       string `json:"state"`
	HeadRefName string `json:"headRefName"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
	Mergeable string `json:"mergeable"`
	Commits   struct {
		Nodes []struct {
			Commit checkCommit `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type checkContextPage struct {
	PageInfo pageInfo `json:"pageInfo"`
	Nodes    []struct {
		Conclusion string `json:"conclusion"`
		State      string `json:"state"`
	} `json:"nodes"`
}

type checkRollup struct {
	Contexts checkContextPage `json:"contexts"`
}

type checkCommit struct {
	ID                string       `json:"id"`
	StatusCheckRollup *checkRollup `json:"statusCheckRollup"`
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
	checks, err := c.checkState(*node)
	if err != nil {
		return PullRequestDetails{}, err
	}
	pr.Checks = checks
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
					PageInfo pageInfo          `json:"pageInfo"`
					Nodes    []pullRequestNode `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		}

		if err := c.graphql.Do(dependabotPRsQuery, variables, &result); err != nil {
			return nil, fmt.Errorf("failed to query pull requests: %w", err)
		}

		for _, node := range result.Repository.PullRequests.Nodes {
			pr := node.toPullRequest()
			if !pr.IsDependabot() {
				continue
			}
			checks, err := c.checkState(node)
			if err != nil {
				return nil, err
			}
			pr.Checks = checks
			prs = append(prs, pr)
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

func (c *client) checkState(pr pullRequestNode) (dependabot.CheckState, error) {
	if len(pr.Commits.Nodes) == 0 {
		return dependabot.CheckNone, nil
	}
	commit := pr.Commits.Nodes[0].Commit
	if commit.StatusCheckRollup == nil {
		return dependabot.CheckNone, nil
	}

	page := commit.StatusCheckRollup.Contexts
	var conclusions []string
	seenCursors := map[string]bool{}
	for {
		for _, check := range page.Nodes {
			conclusion := check.Conclusion
			if conclusion == "" {
				conclusion = mapStateToConclusion(check.State)
			}
			conclusions = append(conclusions, conclusion)
		}
		if !page.PageInfo.HasNextPage {
			return dependabot.CheckStateOf(conclusions), nil
		}

		cursor := page.PageInfo.EndCursor
		if commit.ID == "" || cursor == "" || seenCursors[cursor] {
			return dependabot.CheckNone, fmt.Errorf("failed to query checks for PR #%d: invalid pagination data", pr.Number)
		}
		seenCursors[cursor] = true
		var result struct {
			Node *checkCommit `json:"node"`
		}
		if err := c.graphql.Do(checkContextsQuery, map[string]any{"commit": commit.ID, "after": cursor}, &result); err != nil {
			return dependabot.CheckNone, fmt.Errorf("failed to query checks for PR #%d: %w", pr.Number, err)
		}
		if result.Node == nil || result.Node.StatusCheckRollup == nil {
			return dependabot.CheckNone, fmt.Errorf("failed to query checks for PR #%d: check rollup unavailable", pr.Number)
		}
		page = result.Node.StatusCheckRollup.Contexts
	}
}

func (node pullRequestNode) toPullRequest() dependabot.PullRequest {
	return dependabot.PullRequest{
		Number:      node.Number,
		Title:       node.Title,
		Author:      node.Author.Login,
		HeadRefName: node.HeadRefName,
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
