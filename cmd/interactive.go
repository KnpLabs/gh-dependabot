package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/cli/go-gh/v2"
	"github.com/spf13/cobra"
)

var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Interactively review Dependabot pull requests one by one",
	Long: `Iterate over every open Dependabot pull request, display its diff, and
prompt to approve (y) or skip (n) each one.

The default action shown in the footer flips based on CI status:
  - checks passing/pending: [Y/n] — enter approves
  - any check failing:      [y/N] — enter skips

Skipping has no GitHub side effect: the PR is left untouched and can be
reviewed again on the next run. A summary lists approved and skipped PR
numbers when the loop ends or you quit early.

Pass --merge to also merge each PR after approval, using the method
specified by --method (merge, rebase, squash).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mergeAfterApprove, err := cmd.Flags().GetBool("merge")
		if err != nil {
			return err
		}
		method, err := cmd.Flags().GetString("method")
		if err != nil {
			return err
		}
		deleteBranch, err := cmd.Flags().GetBool("delete-branch")
		if err != nil {
			return err
		}

		var methodFlag string
		if mergeAfterApprove {
			methodFlag, err = mergeMethodFlag(method)
			if err != nil {
				return err
			}
			allowed, err := fetchAllowedMergeMethods()
			if err != nil {
				return err
			}
			if err := validateMergeMethod(method, allowed); err != nil {
				return err
			}
		}

		m := newInteractiveModel()
		m.mergeAfterApprove = mergeAfterApprove
		m.mergeMethodFlag = methodFlag
		m.deleteBranch = deleteBranch

		p := tea.NewProgram(m)
		final, err := p.Run()
		if err != nil {
			return fmt.Errorf("failed to run TUI: %w", err)
		}
		if im, ok := final.(interactiveModel); ok {
			im.printSummary()
		}
		return nil
	},
}

type actionState int

const (
	actionIdle actionState = iota
	actionApproving
	actionMerging
)

type interactiveModel struct {
	prs               []listPullRequest
	index             int
	approved          []int
	merged            []int
	mergeFailed       []prMergeFailure
	skipped           []int
	diff              string
	viewport          viewport.Model
	spinner           spinner.Model
	loadingList       bool
	loadingDiff       bool
	action            actionState
	mergeAfterApprove bool
	mergeMethodFlag   string
	deleteBranch      bool
	err               error
	width             int
	height            int
	done              bool
	quitted           bool
}

type prMergeFailure struct {
	number int
	err    error
}

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
}

func newInteractiveModel() interactiveModel {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	return interactiveModel{
		spinner:     s,
		viewport:    vp,
		loadingList: true,
	}
}

func (m interactiveModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchPRsForInteractive)
}

func fetchPRsForInteractive() tea.Msg {
	prs, err := fetchDependabotPullRequests()
	if err != nil {
		return prsErrorMsg{err: err}
	}
	return prsLoadedMsg{prs: prs}
}

func fetchDiffCmd(number int) tea.Cmd {
	return func() tea.Msg {
		stdout, stderr, err := gh.Exec("pr", "diff", strconv.Itoa(number))
		if err != nil {
			msg := err.Error()
			if stderr.Len() > 0 {
				msg = strings.TrimSpace(stderr.String())
			}
			return diffLoadedMsg{number: number, err: fmt.Errorf("%s", msg)}
		}
		return diffLoadedMsg{number: number, diff: stdout.String()}
	}
}

func approvePRCmd(number int) tea.Cmd {
	return func() tea.Msg {
		_, stderr, err := gh.Exec("pr", "review", strconv.Itoa(number), "--approve")
		if err != nil {
			msg := err.Error()
			if stderr.Len() > 0 {
				msg = strings.TrimSpace(stderr.String())
			}
			return approveDoneMsg{number: number, err: fmt.Errorf("%s", msg)}
		}
		return approveDoneMsg{number: number}
	}
}

func mergePRCmd(number int, methodFlag string, deleteBranch bool) tea.Cmd {
	return func() tea.Msg {
		args := []string{"pr", "merge", strconv.Itoa(number), methodFlag}
		if deleteBranch {
			args = append(args, "--delete-branch")
		}
		_, stderr, err := gh.Exec(args...)
		if err != nil {
			msg := err.Error()
			if stderr.Len() > 0 {
				msg = strings.TrimSpace(stderr.String())
			}
			return mergeDoneMsg{number: number, err: fmt.Errorf("%s", msg)}
		}
		return mergeDoneMsg{number: number}
	}
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
		return m, tea.Batch(m.spinner.Tick, fetchDiffCmd(m.prs[0].Number))

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
		if msg.err != nil {
			m.err = fmt.Errorf("failed to approve PR #%d: %w", msg.number, msg.err)
			return m, tea.Quit
		}
		m.approved = append(m.approved, msg.number)
		if m.mergeAfterApprove {
			m.action = actionMerging
			return m, tea.Batch(m.spinner.Tick, mergePRCmd(msg.number, m.mergeMethodFlag, m.deleteBranch))
		}
		m.action = actionIdle
		return m, m.advance()

	case mergeDoneMsg:
		if msg.err != nil {
			m.mergeFailed = append(m.mergeFailed, prMergeFailure{number: msg.number, err: msg.err})
		} else {
			m.merged = append(m.merged, msg.number)
		}
		m.action = actionIdle
		return m, m.advance()

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	if m.loadingList || m.loadingDiff || m.action != actionIdle {
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
		return m, tea.Quit
	}

	if m.loadingList || m.loadingDiff || m.action != actionIdle || m.index >= len(m.prs) {
		return m, nil
	}

	switch key {
	case "y", "Y":
		m.action = actionApproving
		return m, tea.Batch(m.spinner.Tick, approvePRCmd(m.prs[m.index].Number))
	case "n", "N":
		m.skipped = append(m.skipped, m.prs[m.index].Number)
		return m, m.advance()
	case "enter":
		if defaultApprove(m.prs[m.index]) {
			m.action = actionApproving
			return m, tea.Batch(m.spinner.Tick, approvePRCmd(m.prs[m.index].Number))
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
		return tea.Quit
	}
	m.diff = ""
	m.viewport.SetContent("")
	m.resizeViewport()
	m.loadingDiff = true
	return tea.Batch(m.spinner.Tick, fetchDiffCmd(m.prs[m.index].Number))
}

func (m interactiveModel) View() tea.View {
	if m.err != nil {
		return tea.NewView(fmt.Sprintf("Error: %v\n", m.err))
	}
	if m.loadingList {
		return tea.NewView(fmt.Sprintf("%s Loading dependabot pull requests...", m.spinner.View()))
	}
	if len(m.prs) == 0 {
		return tea.NewView("No open dependabot pull requests found.\n")
	}
	if m.index >= len(m.prs) {
		return tea.NewView("Done.\n")
	}

	var body string
	if m.loadingDiff {
		body = fmt.Sprintf("\n%s Loading diff...\n", m.spinner.View())
	} else {
		body = m.viewport.View()
	}

	v := tea.NewView(fmt.Sprintf("%s\n%s", body, m.bottomBlock()))
	v.AltScreen = true
	return v
}

func (m interactiveModel) bottomBlock() string {
	if len(m.prs) == 0 || m.index >= len(m.prs) {
		return ""
	}
	pr := m.prs[m.index]

	sepWidth := m.width
	if sepWidth < 1 {
		sepWidth = 80
	}
	separator := strings.Repeat("─", sepWidth)

	prefix := fmt.Sprintf("PR %d of %d — #%d  ", m.index+1, len(m.prs), pr.Number)
	prefixCols := utf8.RuneCountInString(prefix)
	titleWidth := sepWidth - prefixCols
	if titleWidth < 20 {
		titleWidth = 20
	}
	titleLines := wrapWords(pr.Title, titleWidth)

	var titleBlock strings.Builder
	titleBlock.WriteString(prefix)
	titleBlock.WriteString(titleLines[0])
	indent := strings.Repeat(" ", prefixCols)
	for _, line := range titleLines[1:] {
		titleBlock.WriteByte('\n')
		titleBlock.WriteString(indent)
		titleBlock.WriteString(line)
	}

	status := fmt.Sprintf("Checks: %s   Mergeable: %s",
		checksStatus(pr.StatusCheckRollup), mergeableStatus(pr.Mergeable))

	var footer string
	switch m.action {
	case actionApproving:
		footer = fmt.Sprintf("%s Approving #%d... (q=quit)", m.spinner.View(), pr.Number)
	case actionMerging:
		footer = fmt.Sprintf("%s Merging #%d... (q=quit)", m.spinner.View(), pr.Number)
	default:
		actionLabel := "Approve"
		yLegend := "y=approve"
		if m.mergeAfterApprove {
			actionLabel = "Approve & merge"
			yLegend = "y=approve+merge"
		}
		var prompt string
		if defaultApprove(pr) {
			prompt = fmt.Sprintf("%s? [Y/n]", actionLabel)
		} else {
			prompt = fmt.Sprintf("%s? [y/N]", actionLabel)
		}
		footer = fmt.Sprintf("%s   (%s, n=skip, ↑↓=scroll, q=quit)", prompt, yLegend)
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s", separator, titleBlock.String(), status, footer)
}

func (m *interactiveModel) resizeViewport() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	bottom := m.bottomBlock()
	bottomLines := 1
	if bottom != "" {
		bottomLines = strings.Count(bottom, "\n") + 1
	}
	m.viewport.SetWidth(m.width)
	m.viewport.SetHeight(maxInt(m.height-bottomLines, 3))
}

func wrapWords(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	var current strings.Builder
	currentCols := 0
	for _, word := range words {
		wordCols := utf8.RuneCountInString(word)
		if current.Len() == 0 {
			current.WriteString(word)
			currentCols = wordCols
			continue
		}
		if currentCols+1+wordCols > width {
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(word)
			currentCols = wordCols
		} else {
			current.WriteByte(' ')
			current.WriteString(word)
			currentCols += 1 + wordCols
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

func (m interactiveModel) printSummary() {
	if m.err != nil {
		return
	}
	if len(m.prs) == 0 {
		fmt.Println("No open dependabot pull requests found.")
		return
	}
	fmt.Println()
	if len(m.approved) > 0 {
		fmt.Printf("Approved (%d): %s\n", len(m.approved), joinNumbers(m.approved))
	} else {
		fmt.Println("Approved (0): —")
	}
	if m.mergeAfterApprove {
		if len(m.merged) > 0 {
			fmt.Printf("Merged   (%d): %s\n", len(m.merged), joinNumbers(m.merged))
		} else {
			fmt.Println("Merged   (0): —")
		}
		if len(m.mergeFailed) > 0 {
			fmt.Printf("Merge failed (%d):\n", len(m.mergeFailed))
			for _, f := range m.mergeFailed {
				fmt.Printf("  #%d: %v\n", f.number, f.err)
			}
		}
	}
	if len(m.skipped) > 0 {
		fmt.Printf("Skipped  (%d): %s\n", len(m.skipped), joinNumbers(m.skipped))
	} else {
		fmt.Println("Skipped  (0): —")
	}
	if m.quitted && m.index < len(m.prs) {
		var remaining []int
		for _, pr := range m.prs[m.index:] {
			remaining = append(remaining, pr.Number)
		}
		fmt.Printf("Unreviewed (%d): %s\n", len(remaining), joinNumbers(remaining))
	}
}

func defaultApprove(pr listPullRequest) bool {
	return checksStatus(pr.StatusCheckRollup) != "✗ failing"
}

func joinNumbers(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = "#" + strconv.Itoa(n)
	}
	return strings.Join(parts, ", ")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func init() {
	interactiveCmd.Flags().Bool("merge", false, "Merge each PR after approving it")
	interactiveCmd.Flags().String("method", "merge", "Merge method to use when --merge is set: merge, rebase, or squash")
	interactiveCmd.Flags().Bool("delete-branch", true, "Delete the branch after merge (only with --merge)")
	rootCmd.AddCommand(interactiveCmd)
}
