package github

import (
	_ "embed"
	"errors"
	"fmt"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

//go:embed merge_approval.graphql
var mergeApprovalQuery string

func (c *client) approvedMergeHead(number int) (string, error) {
	var result struct {
		Repository struct {
			PullRequest *struct {
				BaseRefName              string
				HeadRefName              string
				HeadRefOid               string
				Author                   struct{ Login string }
				LatestOpinionatedReviews struct {
					Nodes []struct {
						State  string
						Commit struct{ Oid string }
					}
				}
			}
		}
	}
	variables := c.repoVariables()
	variables["number"] = number
	if err := c.graphql.Do(mergeApprovalQuery, variables, &result); err != nil {
		return "", fmt.Errorf("failed to verify pull request approval: %w", err)
	}
	pr := result.Repository.PullRequest
	if pr == nil || pr.BaseRefName == "" {
		return "", errors.New("could not verify pull request base branch")
	}
	dependencyPR := dependabot.PullRequest{Author: pr.Author.Login, HeadRefName: pr.HeadRefName}
	if pr.BaseRefName != "develop" || !dependencyPR.IsDependabot() {
		return "", nil
	}
	for _, review := range pr.LatestOpinionatedReviews.Nodes {
		if pr.HeadRefOid != "" && review.State == "APPROVED" && review.Commit.Oid == pr.HeadRefOid {
			return pr.HeadRefOid, nil
		}
	}
	return "", errors.New("Dependabot pull requests targeting develop require approval of the current head commit from a reviewer with write access")
}
