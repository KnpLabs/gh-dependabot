package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
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
		checksStatus(pr.Checks), mergeableStatus(pr.Mergeable))

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
	footer := fmt.Sprintf("%s   (%s, r=rebase, n=skip, ↑↓=scroll, q=quit)", prompt, yLegend)
	if m.pending > 0 {
		footer = fmt.Sprintf("%s %s [%d in-flight]", m.spinner.View(), footer, m.pending)
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
