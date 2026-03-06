package main

import (
	"strings"

	"github.com/rivo/uniseg"
)

// TextInput is a text editing widget that supports both single-line and
// multi-line modes. It replaces bubbles textarea and textinput.
type TextInput struct {
	lines     []string // logical lines (split on \n)
	cursorRow int      // cursor row in lines
	cursorCol int      // cursor column (rune index) in current line
	multiLine bool     // allow newlines via shift+enter / alt+enter
	width     int      // display width (for wrapping / rendering)
	height    int      // visual height (number of display lines to show)
	focused   bool
	echoMode  EchoMode
}

// EchoMode controls how the input text is displayed.
type EchoMode int

const (
	EchoNormal   EchoMode = iota
	EchoPassword          // show asterisks
)

// NewTextInput creates a new text input widget.
func NewTextInput(multiLine bool) *TextInput {
	return &TextInput{
		lines:     []string{""},
		multiLine: multiLine,
		width:     80,
		height:    1,
		focused:   true,
	}
}

// Value returns the full text content with newlines between lines.
func (t *TextInput) Value() string {
	return strings.Join(t.lines, "\n")
}

// SetValue replaces the entire content and moves cursor to end.
func (t *TextInput) SetValue(s string) {
	t.lines = strings.Split(s, "\n")
	if len(t.lines) == 0 {
		t.lines = []string{""}
	}
	t.cursorRow = len(t.lines) - 1
	t.cursorCol = runeLen(t.lines[t.cursorRow])
}

// Reset clears the input.
func (t *TextInput) Reset() {
	t.lines = []string{""}
	t.cursorRow = 0
	t.cursorCol = 0
}

// LineCount returns the number of logical lines.
func (t *TextInput) LineCount() int {
	return len(t.lines)
}

// Width returns the display width.
func (t *TextInput) Width() int {
	return t.width
}

// SetWidth sets the display width.
func (t *TextInput) SetWidth(w int) {
	if w > 0 {
		t.width = w
	}
}

// Height returns the visual height.
func (t *TextInput) Height() int {
	return t.height
}

// SetHeight sets the visual height.
func (t *TextInput) SetHeight(h int) {
	if h > 0 {
		t.height = h
	}
}

// Focus enables the cursor.
func (t *TextInput) Focus() {
	t.focused = true
}

// Blur disables the cursor.
func (t *TextInput) Blur() {
	t.focused = false
}

// Focused returns whether the input is focused.
func (t *TextInput) Focused() bool {
	return t.focused
}

// CursorEnd moves the cursor to the end of the content.
func (t *TextInput) CursorEnd() {
	t.cursorRow = len(t.lines) - 1
	t.cursorCol = runeLen(t.lines[t.cursorRow])
}

// --- Editing operations ---

// InsertRune inserts a single character at the cursor position.
func (t *TextInput) InsertRune(r rune) {
	line := []rune(t.lines[t.cursorRow])
	col := t.clampCol()
	line = append(line[:col], append([]rune{r}, line[col:]...)...)
	t.lines[t.cursorRow] = string(line)
	t.cursorCol = col + 1
}

// InsertText inserts a string (possibly multi-line) at the cursor position.
// Used for paste events and programmatic insertion.
func (t *TextInput) InsertText(s string) {
	if s == "" {
		return
	}
	parts := strings.Split(s, "\n")
	line := []rune(t.lines[t.cursorRow])
	col := t.clampCol()
	before := string(line[:col])
	after := string(line[col:])

	if len(parts) == 1 {
		// Single line paste
		t.lines[t.cursorRow] = before + parts[0] + after
		t.cursorCol = col + runeLen(parts[0])
		return
	}

	// Multi-line paste
	if !t.multiLine {
		// In single-line mode, join all lines with spaces
		joined := strings.Join(parts, " ")
		t.lines[t.cursorRow] = before + joined + after
		t.cursorCol = col + runeLen(joined)
		return
	}

	// First part joins with content before cursor
	t.lines[t.cursorRow] = before + parts[0]

	// Middle parts are new lines
	newLines := make([]string, 0, len(t.lines)+len(parts)-1)
	newLines = append(newLines, t.lines[:t.cursorRow+1]...)
	for _, p := range parts[1 : len(parts)-1] {
		newLines = append(newLines, p)
	}

	// Last part joins with content after cursor
	lastPart := parts[len(parts)-1]
	newLines = append(newLines, lastPart+after)
	newLines = append(newLines, t.lines[t.cursorRow+1:]...)

	t.lines = newLines
	t.cursorRow = t.cursorRow + len(parts) - 1
	t.cursorCol = runeLen(lastPart)
}

// InsertNewline splits the current line at the cursor.
func (t *TextInput) InsertNewline() {
	if !t.multiLine {
		return
	}
	line := []rune(t.lines[t.cursorRow])
	col := t.clampCol()
	before := string(line[:col])
	after := string(line[col:])

	newLines := make([]string, 0, len(t.lines)+1)
	newLines = append(newLines, t.lines[:t.cursorRow]...)
	newLines = append(newLines, before, after)
	newLines = append(newLines, t.lines[t.cursorRow+1:]...)

	t.lines = newLines
	t.cursorRow++
	t.cursorCol = 0
}

// Backspace deletes the character before the cursor.
func (t *TextInput) Backspace() {
	col := t.clampCol()
	if col > 0 {
		line := []rune(t.lines[t.cursorRow])
		t.lines[t.cursorRow] = string(append(line[:col-1], line[col:]...))
		t.cursorCol = col - 1
	} else if t.cursorRow > 0 {
		// Join with previous line
		prevLen := runeLen(t.lines[t.cursorRow-1])
		t.lines[t.cursorRow-1] += t.lines[t.cursorRow]
		t.lines = append(t.lines[:t.cursorRow], t.lines[t.cursorRow+1:]...)
		t.cursorRow--
		t.cursorCol = prevLen
	}
}

// Delete deletes the character at the cursor.
func (t *TextInput) Delete() {
	line := []rune(t.lines[t.cursorRow])
	col := t.clampCol()
	if col < len(line) {
		t.lines[t.cursorRow] = string(append(line[:col], line[col+1:]...))
	} else if t.cursorRow < len(t.lines)-1 {
		// Join with next line
		t.lines[t.cursorRow] += t.lines[t.cursorRow+1]
		t.lines = append(t.lines[:t.cursorRow+1], t.lines[t.cursorRow+2:]...)
	}
}

// DeleteWordBackward deletes the word before the cursor (ctrl+w).
func (t *TextInput) DeleteWordBackward() {
	col := t.clampCol()
	if col == 0 {
		t.Backspace()
		return
	}
	line := []rune(t.lines[t.cursorRow])
	newCol := wordBoundaryLeft(line, col)
	t.lines[t.cursorRow] = string(append(line[:newCol], line[col:]...))
	t.cursorCol = newCol
}

// KillLine deletes from cursor to end of line (ctrl+k).
func (t *TextInput) KillLine() {
	line := []rune(t.lines[t.cursorRow])
	col := t.clampCol()
	t.lines[t.cursorRow] = string(line[:col])
}

// KillToStart deletes from start of line to cursor (ctrl+u).
func (t *TextInput) KillToStart() {
	line := []rune(t.lines[t.cursorRow])
	col := t.clampCol()
	t.lines[t.cursorRow] = string(line[col:])
	t.cursorCol = 0
}

// --- Cursor movement ---

// MoveLeft moves cursor one position left.
func (t *TextInput) MoveLeft() {
	col := t.clampCol()
	if col > 0 {
		t.cursorCol = col - 1
	} else if t.cursorRow > 0 {
		t.cursorRow--
		t.cursorCol = runeLen(t.lines[t.cursorRow])
	}
}

// MoveRight moves cursor one position right.
func (t *TextInput) MoveRight() {
	col := t.clampCol()
	lineLen := runeLen(t.lines[t.cursorRow])
	if col < lineLen {
		t.cursorCol = col + 1
	} else if t.cursorRow < len(t.lines)-1 {
		t.cursorRow++
		t.cursorCol = 0
	}
}

// MoveUp moves cursor one line up.
func (t *TextInput) MoveUp() {
	if t.cursorRow > 0 {
		t.cursorRow--
		t.clampCol() // side effect: clamps cursorCol
	}
}

// MoveDown moves cursor one line down.
func (t *TextInput) MoveDown() {
	if t.cursorRow < len(t.lines)-1 {
		t.cursorRow++
		t.clampCol()
	}
}

// MoveWordLeft moves cursor to the previous word boundary (ctrl+left).
func (t *TextInput) MoveWordLeft() {
	col := t.clampCol()
	if col > 0 {
		t.cursorCol = wordBoundaryLeft([]rune(t.lines[t.cursorRow]), col)
	} else if t.cursorRow > 0 {
		t.cursorRow--
		t.cursorCol = runeLen(t.lines[t.cursorRow])
	}
}

// MoveWordRight moves cursor to the next word boundary (ctrl+right).
func (t *TextInput) MoveWordRight() {
	line := []rune(t.lines[t.cursorRow])
	col := t.clampCol()
	if col < len(line) {
		t.cursorCol = wordBoundaryRight(line, col)
	} else if t.cursorRow < len(t.lines)-1 {
		t.cursorRow++
		t.cursorCol = 0
	}
}

// MoveHome moves cursor to start of line.
func (t *TextInput) MoveHome() {
	t.cursorCol = 0
}

// MoveEnd moves cursor to end of line.
func (t *TextInput) MoveEnd() {
	t.cursorCol = runeLen(t.lines[t.cursorRow])
}

// --- Rendering ---

// View returns the rendered text content as a string.
// The returned string has at most t.height visual lines.
// It does NOT include any border or prompt prefix.
func (t *TextInput) View() string {
	if t.echoMode == EchoPassword {
		return t.viewPassword()
	}
	return t.viewNormal()
}

func (t *TextInput) viewNormal() string {
	// For now, just join lines - wrapping will be handled by the display layer
	// since lipgloss Width on the border handles visual wrapping.
	content := strings.Join(t.lines, "\n")
	if content == "" {
		return ""
	}
	return content
}

func (t *TextInput) viewPassword() string {
	// Show asterisks for the content
	val := t.Value()
	return strings.Repeat("*", runeLen(val))
}

// CursorPosition returns the cursor position (x, y) relative to the rendered
// output. x is the display width of text before the cursor on the current line.
// y is the visual line index accounting for wrapping.
func (t *TextInput) CursorPosition() (x, y int) {
	if !t.focused {
		return 0, 0
	}

	col := t.clampCol()

	if t.echoMode == EchoPassword {
		// All content is on one line as asterisks
		totalRunes := 0
		for i := 0; i < t.cursorRow; i++ {
			totalRunes += runeLen(t.lines[i]) + 1 // +1 for \n
		}
		totalRunes += col
		return totalRunes, 0
	}

	// Count visual lines from wrapped content above cursor row
	y = 0
	for i := 0; i < t.cursorRow; i++ {
		y += t.wrappedLineCount(t.lines[i])
	}

	// Calculate x position and additional y offset from wrapping in current line
	if t.width > 0 {
		curLine := []rune(t.lines[t.cursorRow])
		prefix := string(curLine[:col])
		prefixWidth := uniseg.StringWidth(prefix)
		if t.width > 0 && prefixWidth >= t.width {
			wrapLines := prefixWidth / t.width
			y += wrapLines
			x = prefixWidth - wrapLines*t.width
		} else {
			x = prefixWidth
		}
	} else {
		x = uniseg.StringWidth(string([]rune(t.lines[t.cursorRow])[:col]))
	}

	return x, y
}

// wrappedLineCount returns the number of visual lines a logical line occupies
// when wrapped at the current width.
func (t *TextInput) wrappedLineCount(line string) int {
	if t.width <= 0 {
		return 1
	}
	w := uniseg.StringWidth(line)
	if w == 0 {
		return 1
	}
	n := (w + t.width - 1) / t.width
	return n
}

// DisplayLineCount returns the total number of visual lines across all logical lines.
func (t *TextInput) DisplayLineCount() int {
	total := 0
	for _, line := range t.lines {
		total += t.wrappedLineCount(line)
	}
	return total
}

// --- Helpers ---

// clampCol ensures cursorCol is within the current line's bounds and returns it.
func (t *TextInput) clampCol() int {
	if t.cursorRow < 0 {
		t.cursorRow = 0
	}
	if t.cursorRow >= len(t.lines) {
		t.cursorRow = len(t.lines) - 1
	}
	lineLen := runeLen(t.lines[t.cursorRow])
	if t.cursorCol > lineLen {
		t.cursorCol = lineLen
	}
	if t.cursorCol < 0 {
		t.cursorCol = 0
	}
	return t.cursorCol
}

// wordBoundaryLeft finds the start of the word to the left of col.
func wordBoundaryLeft(line []rune, col int) int {
	if col <= 0 {
		return 0
	}
	i := col - 1
	// Skip spaces
	for i > 0 && isWordSep(line[i]) {
		i--
	}
	// Skip word chars
	for i > 0 && !isWordSep(line[i-1]) {
		i--
	}
	return i
}

// wordBoundaryRight finds the end of the word to the right of col.
func wordBoundaryRight(line []rune, col int) int {
	n := len(line)
	if col >= n {
		return n
	}
	i := col
	// Skip word chars
	for i < n && !isWordSep(line[i]) {
		i++
	}
	// Skip spaces
	for i < n && isWordSep(line[i]) {
		i++
	}
	return i
}

func isWordSep(r rune) bool {
	return r == ' ' || r == '\t' || r == '.' || r == ',' || r == ';' || r == ':' ||
		r == '(' || r == ')' || r == '[' || r == ']' || r == '{' || r == '}' ||
		r == '"' || r == '\'' || r == '/' || r == '\\' || r == '-' || r == '_'
}
