package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/knplabs/gh-dependabot/internal/github"
)

type listModel struct {
	client  github.Client
	table   table.Model
	spinner spinner.Model
	loading bool
	err     error
	prs     []dependabot.PullRequest
}

type prsLoadedMsg struct {
	prs []dependabot.PullRequest
}

type prsErrorMsg struct {
	err error
}

func newListModel(client github.Client) listModel {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))

	columns := []table.Column{
		{Title: "#", Width: 6},
		{Title: "Title", Width: 50},
		{Title: "Checks", Width: 12},
		{Title: "Mergeable", Width: 12},
	}

	tableWidth := 0
	for _, col := range columns {
		tableWidth += col.Width
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(15),
		table.WithWidth(tableWidth),
	)

	return listModel{
		client:  client,
		table:   t,
		spinner: s,
		loading: true,
	}
}

func (m listModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchDependabotPRsCmd(m.client))
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case prsLoadedMsg:
		m.loading = false
		m.prs = msg.prs
		rows := make([]table.Row, len(msg.prs))
		for i, pr := range msg.prs {
			rows[i] = table.Row{
				strconv.Itoa(pr.Number),
				truncateTitle(pr.Title, 48),
				checksStatus(pr.Checks),
				mergeableStatus(pr.Mergeable),
			}
		}
		m.table.SetRows(rows)
		return m, nil

	case prsErrorMsg:
		m.loading = false
		m.err = msg.err
		return m, nil
	}

	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m listModel) View() tea.View {
	var s string
	if m.err != nil {
		s = fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	} else if m.loading {
		s = fmt.Sprintf("%s Loading dependabot pull requests...", m.spinner.View())
	} else if len(m.prs) == 0 {
		s = "No open dependabot pull requests found.\n\nPress q to quit."
	} else {
		s = "Dependabot Pull Requests\n\n" + m.table.View() + "\n\nPress q to quit."
	}
	return tea.NewView(s)
}

func fetchDependabotPRsCmd(client github.Client) tea.Cmd {
	return func() tea.Msg {
		prs, err := client.ListOpenDependabotPRs()
		if err != nil {
			return prsErrorMsg{err: err}
		}
		return prsLoadedMsg{prs: prs}
	}
}

func checksStatus(state dependabot.CheckState) string {
	switch state {
	case dependabot.CheckPassing:
		return "✓ passing"
	case dependabot.CheckFailing:
		return "✗ failing"
	case dependabot.CheckPending:
		return "● pending"
	default:
		return "none"
	}
}

func mergeableStatus(m dependabot.Mergeability) string {
	switch m {
	case dependabot.Mergeable:
		return "✓ yes"
	case dependabot.Conflicting:
		return "✗ conflict"
	default:
		return "● unknown"
	}
}

func truncateTitle(title string, max int) string {
	runes := []rune(title)
	if len(runes) <= max {
		return title
	}
	return strings.TrimSpace(string(runes[:max-1])) + "…"
}
