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
// Accounts for border (2), padding (2), title+blank (2), header (1), separator (1), hint (2).
const modelListChrome = 12 // border + padding + title + header + separator + hint

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

// columnLabel returns the header label for a sort column.
func columnLabel(col sortColumn) string {
	switch col {
	case colName:
		return "MODEL"
	case colProvider:
		return "PROVIDER"
	case colPrice:
		return "PRICE"
	case colSWE:
		return "SWE-BENCH"
	}
	return ""
}

// formatSWEScore formats a SWE-bench score for display.
func formatSWEScore(score float64) string {
	if score == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f", score)
}

// padRight pads s with spaces to width w. If s is longer, it's truncated.
func padRight(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
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

	// Compute column widths from data
	nameW := len("MODEL")
	provW := len("PROVIDER")
	priceW := len("PRICE")
	sweW := len("SWE-BENCH")

	for _, m := range l.models {
		if len(m.DisplayName) > nameW {
			nameW = len(m.DisplayName)
		}
		if len(m.Provider) > provW {
			provW = len(m.Provider)
		}
		p := fmt.Sprintf("%s / %s", formatPrice(m.PromptPrice), formatPrice(m.CompletionPrice))
		if len(p) > priceW {
			priceW = len(p)
		}
		s := formatSWEScore(m.SWEScore)
		if len(s) > sweW {
			sweW = len(s)
		}
	}

	// Add padding between columns
	const colGap = 2

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Bold(true)
	activeHeaderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#B88AFF")).
		Bold(true)
	sepStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3A0066"))
	providerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))
	priceStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555"))
	sweStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9B6ADE"))
	sweNoneStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#444444"))
	activeMarkerStyle := lipgloss.NewStyle().
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

	// Render header row
	sortArrow := func(col sortColumn) string {
		if l.sortCol != col {
			return ""
		}
		if l.sortAsc {
			return " ▲"
		}
		return " ▼"
	}

	renderHeader := func(col sortColumn, width int) string {
		label := columnLabel(col) + sortArrow(col)
		padded := padRight(label, width)
		if l.sortCol == col {
			return activeHeaderStyle.Render(padded)
		}
		return headerStyle.Render(padded)
	}

	gap := strings.Repeat(" ", colGap)
	b.WriteString("  ") // cursor column placeholder
	b.WriteString(renderHeader(colName, nameW))
	b.WriteString(gap)
	b.WriteString(renderHeader(colProvider, provW))
	b.WriteString(gap)
	b.WriteString(renderHeader(colPrice, priceW))
	b.WriteString(gap)
	b.WriteString(renderHeader(colSWE, sweW))
	b.WriteString("\n")

	// Separator line
	totalW := 2 + nameW + colGap + provW + colGap + priceW + colGap + sweW + 3
	b.WriteString(sepStyle.Render("  " + strings.Repeat("─", totalW-2)))
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
		cursorStr := "  "
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

		if i == l.cursor {
			cursorStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#B88AFF")).Render("▸ ")
			labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0")).Bold(true)
		} else {
			cursorStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#3A0066")).Render("  ")
		}

		price := fmt.Sprintf("%s / %s", formatPrice(m.PromptPrice), formatPrice(m.CompletionPrice))
		score := formatSWEScore(m.SWEScore)

		b.WriteString(cursorStr)
		b.WriteString(labelStyle.Render(padRight(m.DisplayName, nameW)))
		b.WriteString(gap)
		b.WriteString(providerStyle.Render(padRight(m.Provider, provW)))
		b.WriteString(gap)
		b.WriteString(priceStyle.Render(padRight(price, priceW)))
		b.WriteString(gap)
		if m.SWEScore == 0 {
			b.WriteString(sweNoneStyle.Render(padRight(score, sweW)))
		} else {
			b.WriteString(sweStyle.Render(padRight(score, sweW)))
		}

		if m.ID == l.activeModel {
			b.WriteString(" ")
			b.WriteString(activeMarkerStyle.Render("●"))
		}
		b.WriteString("\n")
	}

	// Show scroll-down indicator
	if end < len(l.models) {
		b.WriteString(scrollIndicator.Render(fmt.Sprintf("  ↓ %d more", len(l.models)-end)))
		b.WriteString("\n")
	}

	hint := fmt.Sprintf(
		"  %s select  %s cancel  %s sort  %s in/out per 1M tokens",
		hintKeyStyle.Render("enter"),
		hintKeyStyle.Render("esc"),
		hintKeyStyle.Render("←/→"),
		hintKeyStyle.Render("$"),
	)
	b.WriteString(hintStyle.Render(hint))
	b.WriteString("\n")

	return b.String()
}
