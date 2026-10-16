package github

import (
	"fmt"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

const repoMergeMethodsQuery = `query RepoMergeMethods($owner: String!, $name: String!) {
		repository(owner: $owner, name: $name) {
			mergeCommitAllowed
			squashMergeAllowed
			rebaseMergeAllowed
		}
	}`

func (c *client) AllowedMergeMethods() (dependabot.AllowedMergeMethods, error) {
	var result struct {
		Repository struct {
			MergeCommitAllowed bool `json:"mergeCommitAllowed"`
			SquashMergeAllowed bool `json:"squashMergeAllowed"`
			RebaseMergeAllowed bool `json:"rebaseMergeAllowed"`
		} `json:"repository"`
	}
	if err := c.graphql.Do(repoMergeMethodsQuery, c.repoVariables(), &result); err != nil {
		return dependabot.AllowedMergeMethods{}, fmt.Errorf("failed to query repository merge settings: %w", err)
	}
	return dependabot.AllowedMergeMethods{
		Merge:  result.Repository.MergeCommitAllowed,
		Squash: result.Repository.SquashMergeAllowed,
		Rebase: result.Repository.RebaseMergeAllowed,
	}, nil
}
