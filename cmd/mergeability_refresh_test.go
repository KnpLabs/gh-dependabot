package cmd

import (
	"testing"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

func TestMergeabilityRefresh(t *testing.T) {
	prs := []dependabot.PullRequest{
		{Number: 1, Mergeable: dependabot.Mergeable},
		{Number: 2, Mergeable: dependabot.Unknown},
	}
	client := &fakeClient{prs: []dependabot.PullRequest{
		{Number: 2, Mergeable: dependabot.Conflicting},
	}}

	model, refresh := newListModel(client).Update(prsLoadedMsg{prs: prs})
	if refresh == nil {
		t.Fatal("unknown status did not schedule a refresh")
	}
	model, refresh = model.Update(refresh())
	if refresh != nil {
		t.Error("resolved status scheduled another refresh")
	}
	got := model.(listModel).prs
	if got[0] != prs[0] || got[1].Number != 2 || got[1].Mergeable != dependabot.Conflicting {
		t.Errorf("unexpected refreshed PRs: %+v", got)
	}
}
