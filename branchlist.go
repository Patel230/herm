package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// branchList is the UI component for the /branches screen.
type branchList struct {
	items         []string // all branch names
	filtered      []string // branches matching the current filter
	cursor        int
	scroll        int
	filterInput   *TextInput
	currentBranch string // the branch checked out in the current worktree
	width         int
	height        int
	selected      string // set when user presses enter
}

const branchListChrome = 12 // border + padding + title + filter input + hint

func (l branchList) visibleRows() int {
	rows := l.height - branchListChrome
	if rows < 1 {
		rows = 1
	}
	return rows
}

func newBranchList(items []string, currentBranch string, width, height int) branchList {
	ti := NewTextInput(false)
	ti.SetWidth(40)
	ti.Focus()

	bl := branchList{
		items:         items,
		filtered:      items,
		filterInput:   ti,
		currentBranch: currentBranch,
		width:         width,
		height:        height,
	}
	return bl
}

func (l *branchList) clampScroll() {
	vis := l.visibleRows()
	if l.cursor < l.scroll {
		l.scroll = l.cursor
	}
	if l.cursor >= l.scroll+vis {
		l.scroll = l.cursor - vis + 1
	}
	if l.scroll < 0 {
		l.scroll = 0
	}
}

func (l *branchList) applyFilter() {
	query := strings.ToLower(l.filterInput.Value())
	if query == "" {
		l.filtered = l.items
	} else {
		var matches []string
		for _, b := range l.items {
			if strings.Contains(strings.ToLower(b), query) {
				matches = append(matches, b)
			}
		}
		l.filtered = matches
	}
	l.cursor = 0
	l.scroll = 0
}

// HandleKey processes a key event. Returns true if consumed.
func (l *branchList) HandleKey(key EventKey) bool {
	switch key.Key {
	case KeyUp:
		if l.cursor > 0 {
			l.cursor--
			l.clampScroll()
		}
		return true
	case KeyDown:
		if l.cursor < len(l.filtered)-1 {
			l.cursor++
			l.clampScroll()
		}
		return true
	case KeyEnter:
		if len(l.filtered) > 0 && l.cursor < len(l.filtered) {
			l.selected = l.filtered[l.cursor]
		}
		return true
	}

	// Forward to text input for filter typing
	prevValue := l.filterInput.Value()
	handled := l.filterInput.HandleKey(key)
	if l.filterInput.Value() != prevValue {
		l.applyFilter()
	}
	return handled
}

func (l branchList) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#B88AFF")).
		Bold(true).
		PaddingLeft(2).
		PaddingBottom(1)

	if len(l.items) == 0 {
		var b strings.Builder
		b.WriteString(titleStyle.Render("Branches"))
		b.WriteString("\n")
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			PaddingLeft(2)
		b.WriteString(emptyStyle.Render("No branches found."))
		b.WriteString("\n")
		return b.String()
	}

	scrollIndicator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555")).
		PaddingLeft(2)
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555")).
		PaddingLeft(2).
		PaddingTop(1)
	hintKeyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7B3EC7"))

	selectedRowStyle := lipgloss.NewStyle().Background(lipgloss.Color("#2A1545"))

	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7B3EC7"))
	filterTextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
	placeholderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))

	var b strings.Builder
	b.WriteString(titleStyle.Render("Branches"))
	b.WriteString("\n")

	// Filter input
	filterVal := l.filterInput.View()
	if filterVal == "" {
		b.WriteString(promptStyle.Render("  / "))
		b.WriteString(placeholderStyle.Render("Filter branches..."))
	} else {
		b.WriteString(promptStyle.Render("  / "))
		b.WriteString(filterTextStyle.Render(filterVal))
	}
	b.WriteString("\n\n")

	// Empty filter results
	if len(l.filtered) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			PaddingLeft(2)
		b.WriteString(emptyStyle.Render("No matching branches."))
		b.WriteString("\n")
	} else {
		vis := l.visibleRows()
		end := l.scroll + vis
		if end > len(l.filtered) {
			end = len(l.filtered)
		}

		if l.scroll > 0 {
			b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↑ %d more", l.scroll)))
			b.WriteString("\n")
		}

		for i := l.scroll; i < end; i++ {
			branch := l.filtered[i]
			isSelected := i == l.cursor

			var cursorStr string
			nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

			if isSelected {
				cursorStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#B88AFF")).Render("▸ ")
				nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0")).Bold(true)
			} else {
				cursorStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#3A0066")).Render("  ")
			}

			var row strings.Builder
			row.WriteString(nameStyle.Render(branch))

			// Current branch marker
			if branch == l.currentBranch {
				marker := lipgloss.NewStyle().Foreground(lipgloss.Color("#6FE7B8")).Bold(true).Render(" ●")
				row.WriteString(marker)
			}

			b.WriteString(cursorStr)
			if isSelected {
				b.WriteString(selectedRowStyle.Render(row.String()))
			} else {
				b.WriteString(row.String())
			}
			b.WriteString("\n")
		}

		if end < len(l.filtered) {
			b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↓ %d more", len(l.filtered)-end)))
			b.WriteString("\n")
		}
	}

	hint := fmt.Sprintf(
		"  %s filter  %s navigate  %s checkout  %s close",
		hintKeyStyle.Render("type"),
		hintKeyStyle.Render("↑/↓"),
		hintKeyStyle.Render("enter"),
		hintKeyStyle.Render("esc"),
	)
	b.WriteString(hintStyle.Render(hint))
	b.WriteString("\n")

	return b.String()
}
