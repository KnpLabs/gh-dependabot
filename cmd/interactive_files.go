package cmd

import (
	"fmt"
	"path"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/charmbracelet/x/ansi"
)

func (m interactiveModel) sidebarWidth() int {
	if len(m.files) == 0 || m.width < 100 {
		return 0
	}
	return min(m.width/4, 36)
}

func (m interactiveModel) splitDiff() bool {
	return m.sidebarWidth() > 0 && !m.unifiedDiff
}

func (m interactiveModel) fileList(width int) string {
	color := lipgloss.LightDark(m.darkBackground)
	muted := lipgloss.NewStyle().Foreground(color(lipgloss.Color("#606975"), lipgloss.Color("#a4aeba")))
	selected := ansi.Style{}.BackgroundColor(color(lipgloss.Color("#e9edf2"), lipgloss.Color("#2d3540"))).Bold()
	marker := lipgloss.NewStyle().Foreground(color(lipgloss.Color("#855600"), lipgloss.Color("#f0ba61"))).Render("▎")
	row := lipgloss.NewStyle().Width(width)

	var rows []string
	var directory string
	var active int
	for i, file := range m.files {
		name := file.NewName
		if file.IsDelete {
			name = file.OldName
		}
		folder, base := path.Split(name)
		if folder != directory && folder != "" {
			rows = append(rows, row.Render(muted.Render(ansi.Truncate(" "+diffText(folder), width, "…"))))
		}
		directory = folder
		label := fileStatus(file) + " " + diffText(base)
		if i == m.fileIndex {
			active = len(rows)
			rows = append(rows, marker+diffStyled(selected, fitDiffBar(label, diffTotals(file), width-1)))
		} else {
			rows = append(rows, " "+fitDiffBar(label, diffTotals(file), width-1))
		}
	}

	height := m.viewport.Height()
	start := min(max(0, active-height/2), max(0, len(rows)-height))
	list := m.diffBar(fmt.Sprintf(" Files  %d/%d", m.fileIndex+1, len(m.files)), "", width) + "\n" +
		strings.Join(rows[start:min(start+height, len(rows))], "\n")
	return row.Height(height+1).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(color(lipgloss.Color("#c3cbd4"), lipgloss.Color("#47515c"))).
		Render(list)
}

func (m interactiveModel) diffBar(label, stats string, width int) string {
	color := lipgloss.LightDark(m.darkBackground)
	style := ansi.Style{}.
		ForegroundColor(color(lipgloss.Color("#26313c"), lipgloss.Color("#e0e7ef"))).
		BackgroundColor(color(lipgloss.Color("#e9edf2"), lipgloss.Color("#2d3540"))).
		Bold()
	return diffStyled(style, fitDiffBar(label, stats, width))
}

func fitDiffBar(label, stats string, width int) string {
	if stats != "" {
		stats += " "
	}
	available := max(0, width-ansi.StringWidth(stats))
	label = ansi.Truncate(label, available, "…")
	return label + strings.Repeat(" ", available-ansi.StringWidth(label)) + stats
}

func diffTotals(file *gitdiff.File) string {
	var added, deleted int64
	for _, fragment := range file.TextFragments {
		added += fragment.LinesAdded
		deleted += fragment.LinesDeleted
	}
	if added == 0 && deleted == 0 {
		return ""
	}
	plus := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(fmt.Sprintf("+%d", added))
	minus := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(fmt.Sprintf("-%d", deleted))
	return plus + " " + minus
}

func fileStatus(file *gitdiff.File) string {
	switch {
	case file.IsNew:
		return "A"
	case file.IsDelete:
		return "D"
	case file.IsRename:
		return "R"
	case file.IsCopy:
		return "C"
	default:
		return "M"
	}
}
