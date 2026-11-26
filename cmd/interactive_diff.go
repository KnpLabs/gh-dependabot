package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

func diffLines(file *gitdiff.File) []string {
	if file.IsBinary {
		return []string{"Binary file changed."}
	}
	var rows []string
	if file.OldMode != 0 && file.NewMode != 0 && file.OldMode != file.NewMode {
		rows = append(rows, fmt.Sprintf("Mode changed: %s → %s", file.OldMode, file.NewMode))
	}
	width := diffNumberWidth(file)
	section := lipgloss.NewStyle().Bold(true)
	added := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	removed := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	for _, fragment := range file.TextFragments {
		if len(rows) > 0 {
			rows = append(rows, "")
		}
		rows = append(rows, section.Render(diffText(strings.TrimRight(fragment.Header(), "\r\n"))))
		old, newLine := fragment.OldPosition, fragment.NewPosition
		for _, line := range fragment.Lines {
			var before, after string
			if line.Old() {
				before = strconv.FormatInt(old, 10)
				old++
			}
			if line.New() {
				after = strconv.FormatInt(newLine, 10)
				newLine++
			}
			gutter := fmt.Sprintf("%*s %*s │ ", width, before, width, after)
			content := diffText(strings.TrimRight(line.Line, "\r\n"))
			switch line.Op {
			case gitdiff.OpAdd:
				content = added.Render("+ " + content)
			case gitdiff.OpDelete:
				content = removed.Render("- " + content)
			default:
				content = "  " + content
			}
			rows = append(rows, gutter+content)
			if line.NoEOL() {
				rows = append(rows, strings.Repeat(" ", width*2+1)+" │ \\ No newline at end of file")
			}
		}
	}
	if len(rows) == 0 {
		return []string{"No text changes."}
	}
	return rows
}

func diffName(file *gitdiff.File) string {
	switch {
	case file.IsRename, file.IsCopy:
		return diffText(file.OldName + " → " + file.NewName)
	case file.IsDelete:
		return diffText(file.OldName) + " (deleted)"
	case file.IsNew:
		return diffText(file.NewName) + " (added)"
	default:
		return diffText(file.NewName)
	}
}

func diffNumberWidth(file *gitdiff.File) int {
	var last int64
	for _, fragment := range file.TextFragments {
		last = max(last, fragment.OldPosition+fragment.OldLines-1, fragment.NewPosition+fragment.NewLines-1)
	}
	return len(strconv.FormatInt(last, 10))
}

func diffText(s string) string {
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, s)
}
