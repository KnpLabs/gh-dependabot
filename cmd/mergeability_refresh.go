package cmd

import (
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/knplabs/gh-dependabot/internal/github"
)

type mergeabilityRefreshedMsg []dependabot.PullRequest

func refreshMergeability(client github.Client, prs []dependabot.PullRequest, start int) tea.Cmd {
	if !slices.ContainsFunc(prs[start:], func(pr dependabot.PullRequest) bool {
		return pr.Mergeable == dependabot.Unknown
	}) {
		return nil
	}

	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		fresh, err := client.ListOpenDependabotPRs()
		if err != nil {
			return mergeabilityRefreshedMsg(prs)
		}

		updated := slices.Clone(prs)
		for _, pr := range fresh {
			if i := slices.IndexFunc(prs[start:], func(current dependabot.PullRequest) bool {
				return current.Number == pr.Number && current.Mergeable == dependabot.Unknown
			}); i >= 0 {
				updated[start+i].Mergeable = pr.Mergeable
			}
		}
		return mergeabilityRefreshedMsg(updated)
	})
}
