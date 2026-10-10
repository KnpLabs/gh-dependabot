package cmd

import (
	"errors"

	tea "charm.land/bubbletea/v2"
	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/knplabs/gh-dependabot/internal/github"
)

type diffLoadedMsg struct {
	number int
	diff   string
	err    error
}

type approveDoneMsg struct {
	number int
	err    error
}

type mergeDoneMsg struct {
	number int
	err    error
	rebase *rebaseDoneMsg
}

type rebaseDoneMsg struct {
	number int
	err    error
}

func fetchDiffCmd(client github.Client, number int) tea.Cmd {
	return func() tea.Msg {
		diff, err := client.Diff(number)
		return diffLoadedMsg{number: number, diff: diff, err: err}
	}
}

func approvePRCmd(client github.Client, number int) tea.Cmd {
	return func() tea.Msg {
		return approveDoneMsg{number: number, err: client.Approve(number)}
	}
}

func mergePRCmd(client github.Client, number int, method dependabot.MergeMethod, deleteBranch bool) tea.Cmd {
	return func() tea.Msg {
		err := client.Merge(number, method, deleteBranch)
		done := mergeDoneMsg{number: number, err: err}
		if errors.Is(err, github.ErrConflict) {
			done.rebase = &rebaseDoneMsg{number: number, err: client.RequestRebase(number)}
		}
		return done
	}
}

func rebasePRCmd(client github.Client, number int) tea.Cmd {
	return func() tea.Msg {
		return rebaseDoneMsg{number: number, err: client.RequestRebase(number)}
	}
}
