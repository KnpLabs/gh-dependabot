package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

type listPullRequest struct {
	Number            int           `json:"number"`
	Title             string        `json:"title"`
	Author            author        `json:"author"`
	StatusCheckRollup []statusCheck `json:"statusCheckRollup"`
	Mergeable         string        `json:"mergeable"`
}

type listModel struct {
	table   table.Model
	spinner spinner.Model
	loading bool
	err     error
	prs     []listPullRequest
}

type prsLoadedMsg struct {
	prs []listPullRequest
}

type prsErrorMsg struct {
	err error
}

func newListModel() listModel {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))

	columns := []table.Column{
		{Title: "#", Width: 6},
		{Title: "Title", Width: 50},
		{Title: "Checks", Width: 12},
		{Title: "Mergeable", Width: 12},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	return listModel{
		table:   t,
		spinner: s,
		loading: true,
	}
}

func (m listModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchDependabotPRs)
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
				checksStatus(pr.StatusCheckRollup),
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

func fetchDependabotPRs() tea.Msg {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return prsErrorMsg{err: fmt.Errorf("failed to create GraphQL client: %w", err)}
	}

	repo, err := repository.Current()
	if err != nil {
		return prsErrorMsg{err: fmt.Errorf("failed to determine current repository: %w", err)}
	}

	query := `query DependabotPRs($owner: String!, $name: String!) {
		repository(owner: $owner, name: $name) {
			pullRequests(states: OPEN, first: 100) {
				nodes {
					number
					title
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

	variables := map[string]interface{}{
		"owner": repo.Owner,
		"name":  repo.Name,
	}

	var result struct {
		Repository struct {
			PullRequests struct {
				Nodes []struct {
					Number    int    `json:"number"`
					Title     string `json:"title"`
					Author    struct {
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
				} `json:"nodes"`
			} `json:"pullRequests"`
		} `json:"repository"`
	}

	if err := client.Do(query, variables, &result); err != nil {
		return prsErrorMsg{err: fmt.Errorf("failed to query pull requests: %w", err)}
	}

	var dependabotPRs []listPullRequest
	for _, node := range result.Repository.PullRequests.Nodes {
		if !isDependabotAuthor(node.Author.Login) {
			continue
		}

		var checks []statusCheck
		if len(node.Commits.Nodes) > 0 && node.Commits.Nodes[0].Commit.StatusCheckRollup != nil {
			for _, ctx := range node.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes {
				conclusion := ctx.Conclusion
				if conclusion == "" {
					// For StatusContext nodes, map state to conclusion
					conclusion = mapStateToConclusion(ctx.State)
				}
				checks = append(checks, statusCheck{Conclusion: conclusion})
			}
		}

		dependabotPRs = append(dependabotPRs, listPullRequest{
			Number:            node.Number,
			Title:             node.Title,
			Author:            author{Login: node.Author.Login},
			StatusCheckRollup: checks,
			Mergeable:         node.Mergeable,
		})
	}

	return prsLoadedMsg{prs: dependabotPRs}
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

func checksStatus(checks []statusCheck) string {
	if len(checks) == 0 {
		return "none"
	}
	allPass := true
	hasFail := false
	for _, c := range checks {
		switch c.Conclusion {
		case "SUCCESS", "SKIPPED":
			// ok
		case "FAILURE", "ERROR":
			hasFail = true
			allPass = false
		default:
			allPass = false
		}
	}
	if allPass {
		return "✓ passing"
	}
	if hasFail {
		return "✗ failing"
	}
	return "● pending"
}

func mergeableStatus(m string) string {
	switch m {
	case "MERGEABLE":
		return "✓ yes"
	case "CONFLICTING":
		return "✗ conflict"
	default:
		return "● unknown"
	}
}

func truncateTitle(title string, max int) string {
	if len(title) <= max {
		return title
	}
	return strings.TrimSpace(title[:max-1]) + "…"
}
