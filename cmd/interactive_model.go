package cmd

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/knplabs/gh-dependabot/internal/github"
)

type interactiveModel struct {
	client            github.Client
	prs               []dependabot.PullRequest
	index             int
	approved          []int
	approveFailed     []prFailure
	merged            []int
	mergeFailed       []prFailure
	rebased           []int
	rebaseFailed      []prFailure
	skipped           []int
	diff              string
	viewport          viewport.Model
	spinner           spinner.Model
	loadingList       bool
	loadingDiff       bool
	pending           int
	mergeAfterApprove bool
	mergeMethod       dependabot.MergeMethod
	deleteBranch      bool
	err               error
	width             int
	height            int
	done              bool
	quitted           bool
}

type prFailure struct {
	number int
	err    error
}

func newInteractiveModel(client github.Client) interactiveModel {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	return interactiveModel{
		client:      client,
		spinner:     s,
		viewport:    vp,
		loadingList: true,
	}
}

func (m interactiveModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchDependabotPRsCmd(m.client))
}

func (m interactiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()
		return m, nil

	case prsLoadedMsg:
		m.loadingList = false
		m.prs = msg.prs
		if len(m.prs) == 0 {
			m.done = true
			return m, tea.Quit
		}
		m.resizeViewport()
		m.loadingDiff = true
		return m, tea.Batch(m.spinner.Tick, fetchDiffCmd(m.client, m.prs[0].Number))

	case prsErrorMsg:
		m.loadingList = false
		m.err = msg.err
		return m, tea.Quit

	case diffLoadedMsg:
		if m.index >= len(m.prs) || msg.number != m.prs[m.index].Number {
			return m, nil
		}
		m.loadingDiff = false
		if msg.err != nil {
			m.diff = fmt.Sprintf("Failed to load diff: %v", msg.err)
		} else {
			m.diff = msg.diff
		}
		m.viewport.SetContent(m.diff)
		m.viewport.SetYOffset(0)
		return m, nil

	case approveDoneMsg:
		m.pending--
		if msg.err != nil {
			m.approveFailed = append(m.approveFailed, prFailure{number: msg.number, err: msg.err})
		} else {
			m.approved = append(m.approved, msg.number)
			if m.mergeAfterApprove {
				m.pending++
				return m, tea.Batch(m.spinner.Tick, mergePRCmd(m.client, msg.number, m.mergeMethod, m.deleteBranch))
			}
		}
		if m.done && m.pending == 0 {
			return m, tea.Quit
		}
		return m, nil

	case mergeDoneMsg:
		m.pending--
		if msg.err != nil {
			m.mergeFailed = append(m.mergeFailed, prFailure{number: msg.number, err: msg.err})
			if r := msg.rebase; r != nil {
				if r.err != nil {
					m.rebaseFailed = append(m.rebaseFailed, prFailure{number: r.number, err: r.err})
				} else {
					m.rebased = append(m.rebased, r.number)
				}
			}
		} else {
			m.merged = append(m.merged, msg.number)
		}
		if m.done && m.pending == 0 {
			return m, tea.Quit
		}
		return m, nil

	case rebaseDoneMsg:
		m.pending--
		if msg.err != nil {
			m.rebaseFailed = append(m.rebaseFailed, prFailure{number: msg.number, err: msg.err})
		} else {
			m.rebased = append(m.rebased, msg.number)
		}
		if m.done && m.pending == 0 {
			return m, tea.Quit
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	if m.loadingList || m.loadingDiff || m.pending > 0 {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m interactiveModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "ctrl+c", "q":
		m.quitted = true
		m.done = true
		if m.pending == 0 {
			return m, tea.Quit
		}
		return m, m.spinner.Tick
	}

	if m.loadingList || m.loadingDiff || m.index >= len(m.prs) {
		return m, nil
	}

	switch key {
	case "y", "Y":
		number := m.prs[m.index].Number
		m.pending++
		return m, tea.Batch(m.spinner.Tick, approvePRCmd(m.client, number), m.advance())
	case "n", "N":
		m.skipped = append(m.skipped, m.prs[m.index].Number)
		return m, m.advance()
	case "r", "R":
		number := m.prs[m.index].Number
		m.pending++
		return m, tea.Batch(m.spinner.Tick, rebasePRCmd(m.client, number), m.advance())
	case "enter":
		if defaultApprove(m.prs[m.index]) {
			number := m.prs[m.index].Number
			m.pending++
			return m, tea.Batch(m.spinner.Tick, approvePRCmd(m.client, number), m.advance())
		}
		m.skipped = append(m.skipped, m.prs[m.index].Number)
		return m, m.advance()
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *interactiveModel) advance() tea.Cmd {
	m.index++
	if m.index >= len(m.prs) {
		m.done = true
		if m.pending == 0 {
			return tea.Quit
		}
		return m.spinner.Tick
	}
	m.diff = ""
	m.viewport.SetContent("")
	m.resizeViewport()
	m.loadingDiff = true
	return tea.Batch(m.spinner.Tick, fetchDiffCmd(m.client, m.prs[m.index].Number))
}

func defaultApprove(pr dependabot.PullRequest) bool {
	return pr.Checks != dependabot.CheckFailing
}
