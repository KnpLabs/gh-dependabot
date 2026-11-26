package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/charmbracelet/x/ansi"
)

type splitDiffLine struct {
	number    string
	newNumber string
	marker    string
	text      string
	op        gitdiff.LineOp
	noEOL     bool
}

func splitDiffLines(file *gitdiff.File, width int, dark bool) []string {
	color := lipgloss.LightDark(dark)
	muted := lipgloss.NewStyle().Foreground(color(lipgloss.Color("#606975"), lipgloss.Color("#a4aeba")))
	section := muted.Background(color(lipgloss.Color("#eef1f4"), lipgloss.Color("#2a3038"))).Bold(true)
	removed := ansi.Style{}.BackgroundColor(color(lipgloss.Color("#f8e5e7"), lipgloss.Color("#3c2428")))
	added := ansi.Style{}.BackgroundColor(color(lipgloss.Color("#e5f2e7"), lipgloss.Color("#1d3426")))
	removedNumber := removed.ForegroundColor(color(lipgloss.Color("#9b2630"), lipgloss.Color("#ffa8ad"))).Bold()
	addedNumber := added.ForegroundColor(color(lipgloss.Color("#196832"), lipgloss.Color("#a2dfae"))).Bold()
	removedEmphasis := lipgloss.NewStyle().Background(color(lipgloss.Color("#f1c7cd"), lipgloss.Color("#62383f"))).Bold(true)
	addedEmphasis := lipgloss.NewStyle().Background(color(lipgloss.Color("#c5e3ca"), lipgloss.Color("#345341"))).Bold(true)

	rows := make([]string, 0)
	if file.IsBinary {
		rows = append(rows, "Binary file changed.")
	}
	if file.OldMode != 0 && file.NewMode != 0 && file.OldMode != file.NewMode {
		rows = append(rows, fmt.Sprintf("Mode changed: %s → %s", file.OldMode, file.NewMode))
	}
	if file.IsBinary {
		return rows
	}

	lineNumberWidth := diffNumberWidth(file)
	leftWidth := (width - 1) / 2
	rightWidth := width - 1 - leftWidth
	leftContentWidth := max(leftWidth-lineNumberWidth-3, 1)
	rightContentWidth := max(rightWidth-lineNumberWidth-3, 1)
	separator := lipgloss.NewStyle().Foreground(color(lipgloss.Color("#c3cbd4"), lipgloss.Color("#47515c"))).Render("│")
	wrapForColumn := func(s string, contentWidth int) []string {
		return strings.Split(ansi.Hardwrap(s, contentWidth, true), "\n")
	}
	renderSide := func(line *splitDiffLine, parts []string, index, sideWidth int) string {
		if line == nil {
			return strings.Repeat(" ", sideWidth)
		}
		part, number := "", line.number
		if index < len(parts) {
			part = parts[index]
		}
		if index > 0 {
			number = ""
		}
		gutter := fmt.Sprintf("%*s %s ", lineNumberWidth, number, line.marker)
		part += strings.Repeat(" ", max(0, sideWidth-ansi.StringWidth(gutter)-ansi.StringWidth(part)))
		switch line.op {
		case gitdiff.OpDelete:
			return removedNumber.Styled(gutter) + diffStyled(removed, part)
		case gitdiff.OpAdd:
			return addedNumber.Styled(gutter) + diffStyled(added, part)
		default:
			return gutter + part
		}
	}
	appendPaired := func(oldLine, newLine *splitDiffLine) {
		oldParts, newParts := []string{""}, []string{""}
		if oldLine != nil {
			oldParts = wrapForColumn(oldLine.text, leftContentWidth)
		}
		if newLine != nil {
			newParts = wrapForColumn(newLine.text, rightContentWidth)
		}
		if oldLine != nil && newLine != nil && oldLine.op == gitdiff.OpDelete && len(oldParts) == 1 && len(newParts) == 1 {
			oldParts[0], newParts[0] = splitChangedSpan(oldLine.text, newLine.text, removedEmphasis, addedEmphasis)
		}
		for i := range max(len(oldParts), len(newParts)) {
			left := renderSide(oldLine, oldParts, i, leftWidth)
			right := renderSide(newLine, newParts, i, rightWidth)
			rows = append(rows, left+separator+right)
		}
		if (oldLine != nil && oldLine.noEOL) || (newLine != nil && newLine.noEOL) {
			rows = append(rows, muted.Render("\\ No newline at end of file"))
		}
	}

	for _, fragment := range file.TextFragments {
		if len(rows) > 0 {
			rows = append(rows, "")
		}
		heading := diffText(strings.TrimRight(fragment.Header(), "\r\n"))
		for _, line := range strings.Split(ansi.Hardwrap(heading, width, true), "\n") {
			rows = append(rows, section.Render(line+strings.Repeat(" ", max(0, width-ansi.StringWidth(line)))))
		}
		oldPos, newPos := fragment.OldPosition, fragment.NewPosition
		lines := make([]splitDiffLine, 0, len(fragment.Lines))
		for _, line := range fragment.Lines {
			entry := splitDiffLine{text: diffText(strings.TrimRight(line.Line, "\r\n")), op: line.Op, noEOL: line.NoEOL()}
			switch line.Op {
			case gitdiff.OpDelete:
				entry.number, entry.marker = strconv.FormatInt(oldPos, 10), "-"
				oldPos++
			case gitdiff.OpAdd:
				entry.number, entry.marker = strconv.FormatInt(newPos, 10), "+"
				newPos++
			default:
				entry.number, entry.newNumber, entry.marker = strconv.FormatInt(oldPos, 10), strconv.FormatInt(newPos, 10), " "
				oldPos++
				newPos++
			}
			lines = append(lines, entry)
		}
		for i := 0; i < len(lines); {
			if lines[i].op == gitdiff.OpContext {
				oldLine, newLine := lines[i], lines[i]
				newLine.number = newLine.newNumber
				appendPaired(&oldLine, &newLine)
				i++
				continue
			}
			end := i
			for end < len(lines) && lines[end].op != gitdiff.OpContext {
				end++
			}
			var deleted, added []splitDiffLine
			for _, line := range lines[i:end] {
				if line.op == gitdiff.OpDelete {
					deleted = append(deleted, line)
				} else {
					added = append(added, line)
				}
			}
			for j := 0; j < max(len(deleted), len(added)); j++ {
				var before, after *splitDiffLine
				if j < len(deleted) {
					before = &deleted[j]
				}
				if j < len(added) {
					after = &added[j]
				}
				appendPaired(before, after)
			}
			i = end
		}
	}
	if len(rows) == 0 {
		return []string{"No text changes."}
	}
	return rows
}

func splitChangedSpan(oldText, newText string, oldStyle, newStyle lipgloss.Style) (string, string) {
	oldRunes, newRunes := []rune(oldText), []rune(newText)
	prefix := 0
	for prefix < len(oldRunes) && prefix < len(newRunes) && oldRunes[prefix] == newRunes[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldRunes)-prefix && suffix < len(newRunes)-prefix && oldRunes[len(oldRunes)-1-suffix] == newRunes[len(newRunes)-1-suffix] {
		suffix++
	}
	styledSpan := func(s []rune, style lipgloss.Style) string {
		end := len(s) - suffix
		if end == prefix {
			return string(s)
		}
		return string(s[:prefix]) + style.Render(string(s[prefix:end])) + string(s[end:])
	}
	return styledSpan(oldRunes, oldStyle), styledSpan(newRunes, newStyle)
}
