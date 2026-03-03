package main

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// sortColumn identifies which column the model list is sorted by.
type sortColumn int

const (
	colSWE      sortColumn = iota // SWE-bench score (default)
	colName                       // Model name
	colProvider                   // Provider
	colPrice                      // Prompt price
)

const numSortColumns = 4

// modelList is the UI component for the /model selection screen.
type modelList struct {
	models      []ModelDef
	cursor      int
	scroll      int        // index of first visible model row
	activeModel string     // currently active model ID (for highlighting)
	width       int
	height      int
	loading     bool
	sortCol     sortColumn // active sort column
	sortAsc     bool       // true = ascending, false = descending
}

// visibleRows returns how many model rows fit in the available height.
// Accounts for border (2), padding (2), title+blank (3), hint (3).
const modelListChrome = 10 // border top/bottom + padding + title + blank line + hint lines

func (l modelList) visibleRows() int {
	rows := l.height - modelListChrome
	if rows < 1 {
		rows = 1
	}
	return rows
}

func newModelList(models []ModelDef, activeModel string, width, height int) modelList {
	ml := modelList{
		models:      models,
		activeModel: activeModel,
		width:       width,
		height:      height,
		sortCol:     colSWE,
		sortAsc:     false, // SWE-bench: highest first
	}
	ml.sortModels()

	// Find cursor position for active model after sort
	for i, m := range ml.models {
		if m.ID == activeModel {
			ml.cursor = i
			break
		}
	}
	ml.clampScroll()
	return ml
}

// defaultAscending returns the natural sort direction for a column.
func defaultAscending(col sortColumn) bool {
	switch col {
	case colSWE:
		return false // highest score first
	default:
		return true // alphabetical / cheapest first
	}
}

// sortModels sorts the model slice by the current sort column and direction.
func (l *modelList) sortModels() {
	asc := l.sortAsc
	sort.SliceStable(l.models, func(i, j int) bool {
		a, b := l.models[i], l.models[j]
		var less bool
		switch l.sortCol {
		case colName:
			less = strings.ToLower(a.DisplayName) < strings.ToLower(b.DisplayName)
		case colProvider:
			less = strings.ToLower(a.Provider) < strings.ToLower(b.Provider)
		case colPrice:
			less = a.PromptPrice < b.PromptPrice
		case colSWE:
			// Models with no score (0) sort to bottom regardless of direction
			if a.SWEScore == 0 && b.SWEScore == 0 {
				return false
			}
			if a.SWEScore == 0 {
				return false
			}
			if b.SWEScore == 0 {
				return true
			}
			less = a.SWEScore < b.SWEScore
		}
		if asc {
			return less
		}
		return !less
	})
}

// clampScroll ensures the cursor is visible within the scroll window.
func (l *modelList) clampScroll() {
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

func (l modelList) Update(msg tea.Msg) (modelList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height
		l.clampScroll()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if l.cursor > 0 {
				l.cursor--
				l.clampScroll()
			}
		case "down", "j":
			if l.cursor < len(l.models)-1 {
				l.cursor++
				l.clampScroll()
			}
		case "left", "h":
			selectedID := l.models[l.cursor].ID
			l.sortCol = sortColumn((int(l.sortCol) - 1 + numSortColumns) % numSortColumns)
			l.sortAsc = defaultAscending(l.sortCol)
			l.sortModels()
			l.restoreCursor(selectedID)
		case "right", "l":
			selectedID := l.models[l.cursor].ID
			l.sortCol = sortColumn((int(l.sortCol) + 1) % numSortColumns)
			l.sortAsc = defaultAscending(l.sortCol)
			l.sortModels()
			l.restoreCursor(selectedID)
		}
	}
	return l, nil
}

// restoreCursor finds the model with the given ID and moves the cursor to it.
func (l *modelList) restoreCursor(id string) {
	for i, m := range l.models {
		if m.ID == id {
			l.cursor = i
			l.clampScroll()
			return
		}
	}
}

// selected returns the model under the cursor.
func (l modelList) selected() ModelDef {
	return l.models[l.cursor]
}

func (l modelList) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#B88AFF")).
		Bold(true).
		PaddingLeft(2).
		PaddingBottom(1)

	if l.loading {
		var b strings.Builder
		b.WriteString(titleStyle.Render("⚡ Select Model"))
		b.WriteString("\n")
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			PaddingLeft(2)
		b.WriteString(loadingStyle.Render("Loading models..."))
		b.WriteString("\n")
		return b.String()
	}

	providerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true)

	priceStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555"))

	activeMarker := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6FE7B8")).
		Bold(true)

	scrollIndicator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555")).
		PaddingLeft(2)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555")).
		PaddingLeft(2).
		PaddingTop(1)

	hintKeyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7B3EC7"))

	var b strings.Builder
	b.WriteString(titleStyle.Render("⚡ Select Model"))
	b.WriteString("\n")

	vis := l.visibleRows()
	end := l.scroll + vis
	if end > len(l.models) {
		end = len(l.models)
	}

	// Show scroll-up indicator
	if l.scroll > 0 {
		b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↑ %d more", l.scroll)))
		b.WriteString("\n")
	}

	for i := l.scroll; i < end; i++ {
		m := l.models[i]
		cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3A0066"))
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

		cursor := cursorStyle.Render("  ")
		if i == l.cursor {
			cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#B88AFF")).Render("▸ ")
			labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0")).Bold(true)
		}

		b.WriteString(cursor)
		b.WriteString(labelStyle.Render(m.DisplayName))
		b.WriteString("  ")
		b.WriteString(providerStyle.Render(m.Provider))

		// Pricing columns
		if m.PromptPrice > 0 || m.CompletionPrice > 0 {
			b.WriteString("  ")
			b.WriteString(priceStyle.Render(
				fmt.Sprintf("%s / %s", formatPrice(m.PromptPrice), formatPrice(m.CompletionPrice)),
			))
		}

		if m.ID == l.activeModel {
			b.WriteString("  ")
			b.WriteString(activeMarker.Render("●"))
		}
		b.WriteString("\n")
	}

	// Show scroll-down indicator
	if end < len(l.models) {
		b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↓ %d more", len(l.models)-end)))
		b.WriteString("\n")
	}

	hint := fmt.Sprintf(
		"  %s select  %s cancel  %s in/out per 1M tokens",
		hintKeyStyle.Render("enter"),
		hintKeyStyle.Render("esc"),
		hintKeyStyle.Render("$"),
	)
	b.WriteString(hintStyle.Render(hint))
	b.WriteString("\n")

	return b.String()
}
