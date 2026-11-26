package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

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
	if m.done || m.index >= len(m.prs) {
		if m.pending > 0 {
			return tea.NewView(fmt.Sprintf("%s Waiting for %d in-flight request(s) to complete...\n", m.spinner.View(), m.pending))
		}
		return tea.NewView("Done.\n")
	}

	if m.loadingDiff && m.index == 0 {
		return tea.NewView(fmt.Sprintf("%s Loading diff for PR #%d...", m.spinner.View(), m.prs[0].Number))
	}

	v := tea.NewView(fmt.Sprintf("%s\n%s", m.diffView(), m.bottomBlock()))
	v.AltScreen = true
	return v
}

func (m interactiveModel) diffView() string {
	content := m.viewport.View()
	if len(m.files) == 0 {
		return content
	}
	file := m.files[m.fileIndex]
	width := m.viewport.Width()
	if side := m.sidebarWidth(); side > 0 {
		heading := m.diffBar(" "+diffName(file), diffTotals(file), width)
		return lipgloss.JoinHorizontal(lipgloss.Top, m.fileList(side), heading+"\n"+content)
	}
	heading := fmt.Sprintf("File %d/%d  %s", m.fileIndex+1, len(m.files), diffName(file))
	return lipgloss.NewStyle().Bold(true).MaxWidth(width).Render(heading) + "\n" + content
}

func (m interactiveModel) bottomBlock() string {
	if len(m.prs) == 0 || m.index >= len(m.prs) {
		return ""
	}
	index := m.index
	if m.loadingDiff && index > 0 {
		index--
	}
	pr := m.prs[index]

	sepWidth := m.width
	if sepWidth < 1 {
		sepWidth = 80
	}
	separator := strings.Repeat("─", sepWidth)

	prefix := fmt.Sprintf("PR %d of %d — #%d  ", index+1, len(m.prs), pr.Number)
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
		checksStatus(pr.Checks, m.darkBackground), mergeableStatus(pr.Mergeable, m.darkBackground))

	footerLines := m.shortcutRows(pr, sepWidth)
	if m.loadingDiff {
		loading := fmt.Sprintf("%s Loading diff for PR #%d...", m.spinner.View(), m.prs[m.index].Number)
		loadingLines := packShortcuts(sepWidth, loading, shortcut("q", "Quit"))
		footerLines = append(loadingLines, make([]string, max(0, len(footerLines)-len(loadingLines)))...)
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s", separator, titleBlock.String(), status, strings.Join(footerLines, "\n"))
}

func (m interactiveModel) shortcutRows(pr dependabot.PullRequest, width int) []string {
	approveKey, skipKey := "y", "n"
	if defaultApprove(pr) {
		approveKey = "Enter / y"
	} else {
		skipKey = "Enter / n"
	}
	approveLabel := "Approve"
	if m.mergeAfterApprove {
		approveLabel = "Approve & merge"
	}
	actions := []string{shortcut(approveKey, approveLabel), shortcut("r", "Rebase"), shortcut(skipKey, "Skip")}
	if m.pending > 0 {
		actions = append(actions, fmt.Sprintf("%s %d in flight", m.spinner.View(), m.pending))
	}

	navigation := []string{shortcut("↑↓", "Scroll")}
	if len(m.files) > 1 {
		navigation = append(navigation, shortcut("[ ]", "Change file"))
	}
	if m.sidebarWidth() > 0 {
		navigation = append(navigation, shortcut("v", "Switch view"))
	}
	navigation = append(navigation, shortcut("q", "Quit"))
	return append(packShortcuts(width, actions...), packShortcuts(width, navigation...)...)
}

func shortcut(key, label string) string {
	return lipgloss.NewStyle().Bold(true).Render(key) + "  " + label
}

func packShortcuts(width int, entries ...string) []string {
	const gap = "    "
	var rows []string
	var row string
	used := 0
	for _, entry := range entries {
		cells := ansi.StringWidth(entry)
		if row != "" && used+len(gap)+cells > width {
			rows = append(rows, row)
			row, used = "", 0
		}
		if row != "" {
			row += gap
			used += len(gap)
		}
		row += entry
		used += cells
	}
	return append(rows, row)
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
	if len(m.files) > 0 {
		bottomLines++
	}
	width := m.width
	if side := m.sidebarWidth(); side > 0 {
		width -= side + 1
	}
	m.viewport.SetWidth(width)
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

func (m interactiveModel) printSummary(w io.Writer) {
	if m.err != nil {
		return
	}
	if len(m.prs) == 0 {
		fmt.Fprintln(w, "No open dependabot pull requests found.")
		return
	}
	fmt.Fprintln(w)
	if len(m.approved) > 0 {
		fmt.Fprintf(w, "Approved (%d): %s\n", len(m.approved), joinNumbers(m.approved))
	} else {
		fmt.Fprintln(w, "Approved (0): —")
	}
	if len(m.approveFailed) > 0 {
		fmt.Fprintf(w, "Approve failed (%d):\n", len(m.approveFailed))
		for _, f := range m.approveFailed {
			fmt.Fprintf(w, "  #%d: %v\n", f.number, f.err)
		}
	}
	if m.mergeAfterApprove {
		if len(m.merged) > 0 {
			fmt.Fprintf(w, "Merged   (%d): %s\n", len(m.merged), joinNumbers(m.merged))
		} else {
			fmt.Fprintln(w, "Merged   (0): —")
		}
		if len(m.mergeFailed) > 0 {
			fmt.Fprintf(w, "Merge failed (%d):\n", len(m.mergeFailed))
			for _, f := range m.mergeFailed {
				fmt.Fprintf(w, "  #%d: %v\n", f.number, f.err)
			}
		}
	}
	if len(m.rebased) > 0 {
		fmt.Fprintf(w, "Rebased  (%d): %s\n", len(m.rebased), joinNumbers(m.rebased))
	}
	if len(m.rebaseFailed) > 0 {
		fmt.Fprintf(w, "Rebase failed (%d):\n", len(m.rebaseFailed))
		for _, f := range m.rebaseFailed {
			fmt.Fprintf(w, "  #%d: %v\n", f.number, f.err)
		}
	}
	if len(m.skipped) > 0 {
		fmt.Fprintf(w, "Skipped  (%d): %s\n", len(m.skipped), joinNumbers(m.skipped))
	} else {
		fmt.Fprintln(w, "Skipped  (0): —")
	}
	if m.quitted && m.index < len(m.prs) {
		var remaining []int
		for _, pr := range m.prs[m.index:] {
			remaining = append(remaining, pr.Number)
		}
		fmt.Fprintf(w, "Unreviewed (%d): %s\n", len(remaining), joinNumbers(remaining))
	}
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
