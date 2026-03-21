// render.go implements all terminal rendering, visual line wrapping, ANSI
// styling, and screen painting for the herm TUI.
package main

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
	"github.com/rivo/uniseg"
)

// ─── Visual line wrapping (from simple-chat) ───

type vline struct {
	start    int // rune index of first char
	length   int // number of runes
	startCol int // visual column where text starts
}

// getVisualLines splits the input runes into visual lines, accounting for
// the prompt prefix on the first line and terminal-width wrapping.
// It prefers word boundaries (spaces) for wrapping, falling back to
// character-level breaks for words longer than the available width.
func getVisualLines(input []rune, cursor int, width int) []vline {
	var lines []vline
	start := 0
	startCol := promptPrefixCols
	length := 0
	lastSpaceIdx := -1

	for i, r := range input {
		if r == '\n' {
			lines = append(lines, vline{start, length, startCol})
			start = i + 1
			startCol = 0
			length = 0
			lastSpaceIdx = -1
			continue
		}
		length++
		if r == ' ' {
			lastSpaceIdx = i
		}
		if startCol+length >= width {
			if lastSpaceIdx >= start {
				// Word wrap: break after the last space
				wrapLen := lastSpaceIdx - start + 1
				lines = append(lines, vline{start, wrapLen, startCol})
				start = lastSpaceIdx + 1
				length = length - wrapLen
				startCol = 0
				lastSpaceIdx = -1
			} else {
				// No space found, fall back to character-level wrap
				lines = append(lines, vline{start, length, startCol})
				start = i + 1
				startCol = 0
				length = 0
				lastSpaceIdx = -1
			}
		}
	}
	lines = append(lines, vline{start, length, startCol})
	return lines
}

func cursorVisualPos(input []rune, cursor int, width int) (int, int) {
	vlines := getVisualLines(input, cursor, width)
	for i, vl := range vlines {
		end := vl.start + vl.length
		if cursor >= vl.start && cursor <= end {
			// At a line boundary: if this was a soft wrap (not a newline),
			// the cursor belongs on the next line.
			if cursor == end && i < len(vlines)-1 && (end >= len(input) || input[end] != '\n') {
				continue
			}
			return i, vl.startCol + (cursor - vl.start)
		}
	}
	last := len(vlines) - 1
	vl := vlines[last]
	return last, vl.startCol + vl.length
}

// ansiEscRe matches ANSI escape sequences (CSI and OSC).
var ansiEscRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\x1b\].*?\x1b\\`)

// visibleWidth returns the visual column width of s, ignoring ANSI escapes.
func visibleWidth(s string) int {
	return uniseg.StringWidth(ansiEscRe.ReplaceAllString(s, ""))
}

// padCodeBlockRow pads a code block row with spaces so the background fills
// the full terminal width. It ensures \033[0m comes after the padding.
func padCodeBlockRow(row string, width int) string {
	const reset = "\033[0m"
	stripped := row
	hasReset := strings.HasSuffix(row, reset)
	if hasReset {
		stripped = row[:len(row)-len(reset)]
	}
	vw := visibleWidth(stripped)
	if pad := width - vw; pad > 0 {
		stripped += strings.Repeat(" ", pad)
	}
	return stripped + reset
}

// wrapString splits a string into visual rows of at most `w` columns.
// It is ANSI-aware: escape sequences don't count toward visual width, and
// active styling is re-emitted on continuation lines. Character widths are
// measured with uniseg.StringWidth (so wide chars like emoji count as 2).
// Wrapping prefers word boundaries (spaces); words longer than `w` columns
// fall back to character-level breaking.
func wrapString(s string, startCol int, w int) []string {
	if w <= 0 {
		return []string{s}
	}

	// Split into tokens: ANSI sequences and printable segments.
	type token struct {
		text  string
		isSeq bool
	}
	var tokens []token
	rest := s
	for rest != "" {
		loc := ansiEscRe.FindStringIndex(rest)
		if loc == nil {
			tokens = append(tokens, token{rest, false})
			break
		}
		if loc[0] > 0 {
			tokens = append(tokens, token{rest[:loc[0]], false})
		}
		tokens = append(tokens, token{rest[loc[0]:loc[1]], true})
		rest = rest[loc[1]:]
	}

	var rows []string
	var curLine strings.Builder
	col := startCol
	var activeSeqs []string // stack of active ANSI sequences for re-emit

	flush := func() {
		rows = append(rows, curLine.String())
		curLine.Reset()
		col = 0
		for _, seq := range activeSeqs {
			curLine.WriteString(seq)
		}
	}

	applyANSI := func(seq string) {
		curLine.WriteString(seq)
		if seq == "\033[0m" || seq == "\033[m" {
			activeSeqs = nil
		} else {
			activeSeqs = append(activeSeqs, seq)
		}
	}

	// Word buffer: accumulates parts (text chunks and ANSI sequences) that
	// form a single visual word spanning across tokens.
	type wordPart struct {
		text  string
		isSeq bool
	}
	var wordParts []wordPart
	var wordBuf strings.Builder // accumulates current run of non-space chars
	wordWidth := 0

	flushWordBuf := func() {
		if wordBuf.Len() > 0 {
			wordParts = append(wordParts, wordPart{wordBuf.String(), false})
			wordBuf.Reset()
		}
	}

	commitWord := func() {
		flushWordBuf()
		if wordWidth == 0 {
			// Only ANSI sequences — apply them directly
			for _, p := range wordParts {
				if p.isSeq {
					applyANSI(p.text)
				}
			}
			wordParts = wordParts[:0]
			return
		}
		if wordWidth <= w {
			// Word fits on a full line — move to next line if needed
			if col+wordWidth > w {
				flush()
			}
			for _, p := range wordParts {
				if p.isSeq {
					applyANSI(p.text)
				} else {
					curLine.WriteString(p.text)
				}
			}
			col += wordWidth
		} else {
			// Word wider than line — character-break
			for _, p := range wordParts {
				if p.isSeq {
					applyANSI(p.text)
				} else {
					for _, r := range p.text {
						rw := uniseg.StringWidth(string(r))
						if col+rw > w {
							flush()
						}
						curLine.WriteRune(r)
						col += rw
					}
				}
			}
		}
		wordParts = wordParts[:0]
		wordWidth = 0
	}

	for _, tok := range tokens {
		if tok.isSeq {
			flushWordBuf()
			wordParts = append(wordParts, wordPart{tok.text, true})
			continue
		}
		for _, r := range tok.text {
			if unicode.IsSpace(r) {
				commitWord()
				rw := uniseg.StringWidth(string(r))
				if col+rw > w {
					flush()
				} else {
					curLine.WriteRune(r)
					col += rw
				}
			} else {
				wordBuf.WriteRune(r)
				wordWidth += uniseg.StringWidth(string(r))
			}
		}
	}
	commitWord()
	rows = append(rows, curLine.String())

	if len(rows) == 0 {
		return []string{""}
	}
	return rows
}

// ─── Progress bar (from simple-chat) ───

func lerpColor(r1, g1, b1, r2, g2, b2 int, t float64) (int, int, int) {
	lerp := func(a, b int) int { return a + int(float64(b-a)*t) }
	return lerp(r1, r2), lerp(g1, g2), lerp(b1, b2)
}

func progressBar(n, max int) string {
	if n > max {
		n = max
	}
	ratio := float64(n) / float64(max)
	filled := int(ratio * 24)
	partials := []rune("█▉▊▋▌▍▎▏")

	r, g, b := lerpColor(78, 201, 100, 230, 70, 70, ratio)
	fillFg := fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
	dimBg := "\033[48;5;240m"

	const reset = "\033[0m"

	var buf strings.Builder
	for i := range 3 {
		cellFilled := filled - i*8
		switch {
		case cellFilled >= 8:
			buf.WriteString(dimBg + fillFg + "█")
		case cellFilled <= 0:
			buf.WriteString(dimBg + " ")
		default:
			buf.WriteString(dimBg + fillFg + string(partials[8-cellFilled]))
		}
	}
	buf.WriteString(reset)
	return buf.String()
}

// ─── ANSI rendering helpers (from simple-chat) ───

func writeRows(buf *strings.Builder, rows []string, from int) {
	if len(rows) == 0 {
		return
	}
	buf.WriteString(fmt.Sprintf("\033[%d;1H", from))
	for i, row := range rows {
		if i > 0 {
			buf.WriteString("\r\n")
		}
		buf.WriteString("\033[0m\033[2K")
		buf.WriteString(row)
	}
}

// ─── Logo ───

// buildLogo returns the colored logo lines.
// Shell body uses two warm sand tones, eyes are black,
// and the interior (face area) has a grey-blue background.
// HERM acronym rendered as 2-row block art with cyan→pink gradient.
func buildLogo(width int) []string {
	shA := "\033[38;5;180m" // shell body (warm sand)
	shB := "\033[38;5;223m" // shell highlight (lighter peach)
	ib := "\033[48;5;60m"   // interior bg (grey-blue)
	ey := "\033[38;5;232m"  // black eyes
	tb := "\033[48;5;60m"   // tentacle bg
	r := "\033[0m"
	d := "\033[2m" // dim (tagline)
	// HERM gradient: cyan → blue-purple → purple → hot pink
	cH := "\033[1;38;2;0;212;255m"
	cE := "\033[1;38;2;85;140;230m"
	cR := "\033[1;38;2;170;68;200m"
	cM := "\033[1;38;2;255;20;147m"
	prefix := " " + shA + "▀" + shB + "██" + shA + "█" + tb + shA + "▌▌▌" + r + shA + "█" + r + "  " + d
	tagline := "Helpful Encapsulated Reasoning Machine"
	prefixWidth := visibleWidth(prefix)
	available := width - prefixWidth
	if available < len(tagline) && available > 1 {
		tagline = tagline[:available-1] + "…"
	} else if available <= 1 {
		tagline = ""
	}
	return []string{
		"",
		"    " + shA + "▄" + shB + "███" + shA + "▄" + r + "  " + cH + "█ █" + r + " " + cE + "█▀▀" + r + " " + cR + "█▀█" + r + " " + cM + "█▄ ▄█" + r,
		"  " + shA + "▄██" + ib + ey + "• •" + r + shA + "█" + r + "  " + cH + "█▀█" + r + " " + cE + "██▄" + r + " " + cR + "█▀▄" + r + " " + cM + "█ ▀ █" + r,
		prefix + tagline + r,
		"",
	}
}

// ─── Styling helpers ───

func styledUserMsg(content string) string {
	// Style each line individually so \n splits in buildBlockRows preserve it.
	lines := strings.Split(renderInlineMarkdown(content), "\n")
	lines[0] = "\033[1m▸ " + lines[0] + "\033[0m"
	for i := 1; i < len(lines); i++ {
		lines[i] = "\033[1m" + lines[i] + "\033[0m"
	}
	return strings.Join(lines, "\n")
}

func styledAssistantText(content string) string {
	return content
}

func styledToolCall(summary string) string {
	return "\033[2;3m" + summary + "\033[0m"
}

func styledToolResult(result string, isError bool) string {
	if isError {
		return styledError(result)
	}
	return "\033[2m" + result + "\033[0m"
}

func styledError(msg string) string {
	return "\033[31;3m" + msg + "\033[0m"
}

func styledSuccess(msg string) string {
	return "\033[32;3m" + msg + "\033[0m"
}

func styledInfo(msg string) string {
	return "\033[34;3m" + msg + "\033[0m"
}

func styledSystemPrompt(msg string) string {
	// dim italic — same style as tool calls / thinking indicator.
	// Style each line individually so \n splits in buildBlockRows preserve it.
	lines := strings.Split(msg, "\n")
	for i, line := range lines {
		lines[i] = "\033[2;3m" + line + "\033[0m"
	}
	return strings.Join(lines, "\n")
}

func renderMessage(msg chatMessage) string {
	var parts []string
	if msg.leadBlank {
		parts = append(parts, "")
	}
	// Strip carriage returns to prevent terminal cursor jumps that garble output.
	content := strings.ReplaceAll(msg.content, "\r", "")
	var rendered string
	switch msg.kind {
	case msgUser:
		rendered = styledUserMsg(content)
	case msgAssistant:
		rendered = styledAssistantText(content)
	case msgToolCall:
		rendered = styledToolCall(content)
	case msgToolResult:
		rendered = styledToolResult(content, msg.isError)
	case msgInfo:
		rendered = styledInfo(content)
	case msgSystemPrompt:
		rendered = styledSystemPrompt(content)
	case msgSuccess:
		rendered = styledSuccess(content)
	case msgError:
		rendered = styledError(content)
	}
	parts = append(parts, rendered)
	return strings.Join(parts, "\n")
}

// renderToolBox renders a tool call and its result as a bordered box:
//
//	┌ ~ glob ───────┐
//	file1.go
//	file2.go
//	└───────────────┘
//
// The box has top/bottom borders but no side borders. The entire output is
// styled dim (or red for errors). Title uses dim+italic.
func renderToolBox(title, content string, maxWidth int, isError bool, durationStr string) string {
	// Replace tabs with single spaces for compact, predictable display.
	content = strings.ReplaceAll(content, "\t", " ")
	// Compute inner width from title and content lines.
	titleVW := visibleWidth(title)
	innerWidth := titleVW + 2 // "┌ " + title + " " + pad + "┐" → need at least title + 2 spaces
	if content != "" {
		for _, line := range strings.Split(content, "\n") {
			if lw := visibleWidth(line); lw > innerWidth {
				innerWidth = lw
			}
		}
	}
	// Ensure inner width is wide enough for the duration label if present.
	// Bottom border: └─── duration ┘ → needs len(duration) + 2 (spaces around it).
	if durationStr != "" {
		if minW := len(durationStr) + 2; minW > innerWidth {
			innerWidth = minW
		}
	}
	// Cap at maxWidth minus 2 for corner characters (┌/┐ are each 1 wide).
	if maxWidth > 0 && innerWidth > maxWidth-2 {
		innerWidth = maxWidth - 2
	}
	// Truncate title if it doesn't fit within the capped inner width.
	// The top border is "┌ title ─┐", so title needs innerWidth - 2 visible chars.
	if maxTitleVW := innerWidth - 2; titleVW > maxTitleVW && maxTitleVW >= 0 {
		title = truncateWithEllipsis(title, maxTitleVW)
		titleVW = visibleWidth(title)
	}

	// Pick ANSI style for borders vs content.
	var borderStyle, titleStyle, contentStyle, reset string
	if isError {
		borderStyle = "\033[31m"   // red
		titleStyle = "\033[31;3m"  // red italic
		contentStyle = "\033[31m"  // red
		reset = "\033[0m"
	} else {
		borderStyle = "\033[2m"    // dim
		titleStyle = "\033[2;3m"   // dim italic
		contentStyle = "\033[2m"   // dim
		reset = "\033[0m"
	}

	var b strings.Builder

	// Top border: ┌ title ─...─┐
	pad := innerWidth - titleVW - 2 // spaces taken by " title "
	if pad < 0 {
		pad = 0
	}
	b.WriteString(borderStyle)
	b.WriteString("┌ ")
	b.WriteString(reset)
	b.WriteString(titleStyle)
	b.WriteString(title)
	b.WriteString(reset)
	b.WriteString(borderStyle)
	b.WriteByte(' ')
	b.WriteString(strings.Repeat("─", pad))
	b.WriteString("┐")
	b.WriteString(reset)

	// Content lines (no side borders).
	if content != "" {
		isDiff := isDiffContent(content)
		for _, line := range strings.Split(content, "\n") {
			b.WriteByte('\n')
			lineStyle := contentStyle
			if isDiff {
				if ds := diffLineStyle(line); ds != "" {
					lineStyle = ds
				}
			}
			b.WriteString(lineStyle)
			if visibleWidth(line) > innerWidth {
				line = truncateVisual(line, innerWidth)
			}
			b.WriteString(line)
			b.WriteString(reset)
		}
	}

	// Bottom border: └─...─┘ or └─...─ 1.2s ┘
	b.WriteByte('\n')
	b.WriteString(borderStyle)
	b.WriteString("└")
	if durationStr != "" {
		durPad := innerWidth - len(durationStr) - 2 // " duration "
		if durPad < 0 {
			durPad = 0
		}
		b.WriteString(strings.Repeat("─", durPad))
		b.WriteByte(' ')
		b.WriteString(reset)
		b.WriteString(titleStyle)
		b.WriteString(durationStr)
		b.WriteString(reset)
		b.WriteString(borderStyle)
		b.WriteString(" ┘")
	} else {
		b.WriteString(strings.Repeat("─", innerWidth))
		b.WriteString("┘")
	}
	b.WriteString(reset)

	return b.String()
}

var funnyTexts = []string{
	"pondering the cosmos...",
	"consulting the oracle...",
	"herding electrons...",
	"untangling spaghetti...",
	"asking the rubber duck...",
	"dividing by zero...",
	"reticulating splines...",
	"compiling thoughts...",
	"traversing the astral plane...",
	"shaking the magic 8-ball...",
	"feeding the hamsters...",
	"polishing pixels...",
	"summoning the muse...",
	"counting backwards from infinity...",
	"aligning the chakras...",
	"brewing coffee virtually...",
	"negotiating with the compiler...",
	"reading the tea leaves...",
}

// hslToRGB converts HSL (h in [0,360), s and l in [0,1]) to RGB [0,255].
func hslToRGB(h, s, l float64) (int, int, int) {
	c := (1 - math.Abs(2*l-1)) * s
	hp := h / 60
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var r1, g1, b1 float64
	switch {
	case hp < 1:
		r1, g1, b1 = c, x, 0
	case hp < 2:
		r1, g1, b1 = x, c, 0
	case hp < 3:
		r1, g1, b1 = 0, c, x
	case hp < 4:
		r1, g1, b1 = 0, x, c
	case hp < 5:
		r1, g1, b1 = x, 0, c
	default:
		r1, g1, b1 = c, 0, x
	}
	m := l - c/2
	return int(math.Round((r1 + m) * 255)),
		int(math.Round((g1 + m) * 255)),
		int(math.Round((b1 + m) * 255))
}

// pastelColor returns an ANSI true-color escape for a smoothly cycling pastel hue.
func pastelColor(elapsed time.Duration) string {
	hue := math.Mod(elapsed.Seconds()*90, 360) // full rotation every 4s
	r, g, b := hslToRGB(hue, 0.65, 0.78)
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
}

// approvalGradientColor returns a bold ANSI true-color escape cycling through
// saturated yellow/amber/gold tones so the approval prompt is impossible to miss.
func approvalGradientColor(t time.Duration) string {
	phase := math.Sin(t.Seconds() * 2 * math.Pi / 1.5)
	hue := 42.5 + 12.5*phase // oscillate 30..55 (gold to yellow)
	r, g, b := hslToRGB(hue, 0.95, 0.52)
	return fmt.Sprintf("\033[1;38;2;%d;%d;%dm", r, g, b)
}

// approvalGradientSep returns a separator line where each dash character is
// individually colored with a shifting yellow gradient wave effect.
func approvalGradientSep(width int, t time.Duration) string {
	var buf strings.Builder
	for i := 0; i < width; i++ {
		charPhase := math.Sin((t.Seconds()*2*math.Pi/1.5) + float64(i)*0.15)
		hue := 42.5 + 12.5*charPhase
		r, g, b := hslToRGB(hue, 0.95, 0.52)
		fmt.Fprintf(&buf, "\033[1;38;2;%d;%d;%dm─", r, g, b)
	}
	buf.WriteString("\033[0m")
	return buf.String()
}

// wrapLineCount returns the number of visual lines that `line` would occupy
// when word-wrapped to `width` columns. It delegates to wrapString.
func wrapLineCount(line string, width int) int {
	return len(wrapString(line, 0, width))
}

func (a *App) buildBlockRows() []string {
	var rows []string
	for _, line := range buildLogo(a.width) {
		rows = append(rows, wrapString(line, 0, a.width)...)
	}
	inCodeBlock := false
	skipNext := false
	for i, msg := range a.messages {
		if skipNext {
			skipNext = false
			// Still emit trailing blank line logic below.
			goto blankLine
		}

		// Tool call + result pair → render as a single box.
		if msg.kind == msgToolCall {
			title := strings.ReplaceAll(msg.content, "\r", "")
			nextIdx := i + 1
			if nextIdx < len(a.messages) && a.messages[nextIdx].kind == msgToolResult {
				// Paired: render full box.
				result := a.messages[nextIdx]
				content := strings.ReplaceAll(result.content, "\r", "")
				box := renderToolBox(title, content, a.width, result.isError, formatDuration(result.duration))
				if msg.leadBlank {
					rows = append(rows, "")
				}
				for _, logLine := range strings.Split(box, "\n") {
					rows = append(rows, wrapString(logLine, 0, a.width)...)
				}
				skipNext = true
				goto blankLine
			}
			// Unpaired (in-progress): show open box, or full box with live timer after 500ms.
			if msg.leadBlank {
				rows = append(rows, "")
			}
			var liveDur string
			if !a.toolStartTime.IsZero() {
				liveDur = formatDuration(time.Since(a.toolStartTime))
			}
			box := renderToolBox(title, "", a.width, false, liveDur)
			if liveDur == "" {
				// Under 500ms: strip bottom border (open box).
				boxLines := strings.Split(box, "\n")
				if len(boxLines) > 1 {
					boxLines = boxLines[:len(boxLines)-1] // remove └...┘
				}
				for _, logLine := range boxLines {
					rows = append(rows, wrapString(logLine, 0, a.width)...)
				}
			} else {
				// Over 500ms: show full box with live duration.
				for _, logLine := range strings.Split(box, "\n") {
					rows = append(rows, wrapString(logLine, 0, a.width)...)
				}
			}
			goto blankLine
		}

		// Tool result without preceding tool call — render as a standalone box.
		if msg.kind == msgToolResult {
			content := strings.ReplaceAll(msg.content, "\r", "")
			box := renderToolBox("~ result", content, a.width, msg.isError, formatDuration(msg.duration))
			if msg.leadBlank {
				rows = append(rows, "")
			}
			for _, logLine := range strings.Split(box, "\n") {
				rows = append(rows, wrapString(logLine, 0, a.width)...)
			}
			goto blankLine
		}

		{
			rendered := renderMessage(msg)
			for _, logLine := range strings.Split(rendered, "\n") {
				wasInCodeBlock := inCodeBlock
				if msg.kind == msgAssistant {
					var skip bool
					logLine, inCodeBlock, skip = processMarkdownLine(logLine, inCodeBlock)
					if skip {
						continue
					}
				}
				wrapped := wrapString(logLine, 0, a.width)
				if wasInCodeBlock && msg.kind == msgAssistant {
					for j := range wrapped {
						wrapped[j] = padCodeBlockRow(wrapped[j], a.width)
					}
				}
				rows = append(rows, wrapped...)
			}
		}

	blankLine:
		// Add blank line after block, unless:
		// - next message already has leadBlank, or
		// - this is an assistant message followed by another assistant message
		//   (consecutive assistant chunks already contain their own newlines)
		// When we consumed a pair (skipNext was just set), look past the result.
		peekIdx := i + 1
		if skipNext {
			peekIdx = i + 2
		}
		peekHasBlank := peekIdx < len(a.messages) && a.messages[peekIdx].leadBlank
		peekIsAssistant := peekIdx < len(a.messages) && a.messages[peekIdx].kind == msgAssistant
		if !peekHasBlank && !(msg.kind == msgAssistant && peekIsAssistant) {
			rows = append(rows, "")
		}
	}
	// Show streaming text above the input area
	if a.streamingText != "" {
		for _, logLine := range strings.Split(a.streamingText, "\n") {
			wasInCodeBlock := inCodeBlock
			var skip bool
			logLine, inCodeBlock, skip = processMarkdownLine(logLine, inCodeBlock)
			if !skip {
				wrapped := wrapString(logLine, 0, a.width)
				if wasInCodeBlock {
					for j := range wrapped {
						wrapped[j] = padCodeBlockRow(wrapped[j], a.width)
					}
				}
				rows = append(rows, wrapped...)
			}
		}
		rows = append(rows, "")
	}
	// Show animated status line while agent is running, or dim elapsed when done
	if a.agentRunning && a.awaitingApproval {
		// Paused: show dim elapsed while waiting for user approval
		elapsed := a.agentElapsedTime()
		label := fmt.Sprintf("\033[2;3m⏸ %.2fs ↑%s ↓%s\033[0m",
			elapsed.Seconds(),
			formatTokenCount(int(math.Round(a.agentDisplayInTok))),
			formatTokenCount(int(math.Round(a.agentDisplayOutTok))))
		rows = append(rows, wrapString(label, 0, a.width)...)
		rows = append(rows, "")
	} else if a.agentRunning {
		elapsed := a.agentElapsedTime()
		text := funnyTexts[a.agentTextIndex]
		color := pastelColor(elapsed)
		label := fmt.Sprintf("%s\033[3m%s %.2fs ↑%s ↓%s\033[0m",
			color, text, elapsed.Seconds(),
			formatTokenCount(int(math.Round(a.agentDisplayInTok))),
			formatTokenCount(int(math.Round(a.agentDisplayOutTok))))
		rows = append(rows, wrapString(label, 0, a.width)...)
		rows = append(rows, "")
	} else if a.agentElapsed > 0 {
		elapsed := fmt.Sprintf("\033[2m%.2fs ↑%s ↓%s\033[0m",
			a.agentElapsed.Seconds(),
			formatTokenCount(a.mainAgentInputTokens),
			formatTokenCount(a.mainAgentOutputTokens))
		rows = append(rows, wrapString(elapsed, 0, a.width)...)
		rows = append(rows, "")
	}
	// Show live sub-agent activity (capped to 3 lines, dim/italic)
	if subLines := a.subAgentDisplayLines(); len(subLines) > 0 {
		for _, line := range subLines {
			rows = append(rows, wrapString(line, 0, a.width)...)
		}
		rows = append(rows, "")
	}
	return collapseBlankRows(rows)
}

// collapseBlankRows reduces consecutive blank rows to at most one.
// A row is "blank" if it is empty or contains only ANSI reset sequences.
func collapseBlankRows(rows []string) []string {
	out := make([]string, 0, len(rows))
	prevBlank := false
	for _, r := range rows {
		blank := isBlankRow(r)
		if blank && prevBlank {
			continue
		}
		out = append(out, r)
		prevBlank = blank
	}
	return out
}

// isBlankRow reports whether a row is visually empty (empty string or only ANSI escapes).
func isBlankRow(s string) bool {
	return strings.TrimSpace(ansiEscRe.ReplaceAllString(s, "")) == ""
}

// subAgentDisplay tracks per-agent display state for live TUI rendering.
type subAgentDisplay struct {
	task   string // task label (first ~40 chars of the task description)
	status string // current activity (tool name or text snippet)
	done   bool
}

// maxSubAgentDisplayLines is the maximum number of active agent lines shown.
const maxSubAgentDisplayLines = 5

// subAgentDisplayLines returns one line per active sub-agent showing its task label and status.
func (a *App) subAgentDisplayLines() []string {
	if len(a.subAgents) == 0 {
		return nil
	}
	var active []*subAgentDisplay
	for _, sa := range a.subAgents {
		if !sa.done {
			active = append(active, sa)
		}
	}
	if len(active) == 0 {
		return nil
	}
	var out []string
	shown := active
	if len(shown) > maxSubAgentDisplayLines {
		shown = shown[:maxSubAgentDisplayLines]
	}
	for _, sa := range shown {
		label := sa.task
		status := sa.status
		if status == "" {
			status = "starting..."
		}
		// dim (2) + italic (3), task label in normal weight
		out = append(out, fmt.Sprintf("\033[2;3m[agent] \033[0;2m%s\033[2;3m: %s\033[0m", label, status))
	}
	if len(active) > maxSubAgentDisplayLines {
		out = append(out, fmt.Sprintf("\033[2;3m  ...and %d more\033[0m", len(active)-maxSubAgentDisplayLines))
	}
	return out
}

// truncateTaskLabel returns the first ~40 chars of a task description for display.
func truncateTaskLabel(task string) string {
	// Take first line only.
	if idx := strings.IndexByte(task, '\n'); idx >= 0 {
		task = task[:idx]
	}
	const maxLen = 40
	if len(task) > maxLen {
		task = task[:maxLen] + "…"
	}
	return task
}

// shortID returns the first 8 characters of an agent ID for display.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// getOrCreateSubAgent returns the display state for the given agent ID, creating it if needed.
func (a *App) getOrCreateSubAgent(agentID string) *subAgentDisplay {
	if a.subAgents == nil {
		a.subAgents = make(map[string]*subAgentDisplay)
	}
	sa, ok := a.subAgents[agentID]
	if !ok {
		sa = &subAgentDisplay{task: "unknown task"}
		a.subAgents[agentID] = sa
	}
	return sa
}

func (a *App) buildInputRows() []string {
	sep := strings.Repeat("─", a.width)

	// Approval mode: animated yellow gradient borders + centered message
	if a.awaitingApproval {
		t := time.Since(a.approvalPauseStart)
		color := approvalGradientColor(t)
		shortMsg := fmt.Sprintf("Allow %s? [y/n]", a.approvalSummary)
		if len(shortMsg) > a.width {
			shortMsg = shortMsg[:a.width]
		}
		shortPad := (a.width - len(shortMsg)) / 2
		if shortPad < 0 {
			shortPad = 0
		}
		detail := a.approvalDesc
		if detail == a.approvalSummary {
			detail = ""
		}
		approvalRows := []string{sep}
		approvalRows = append(approvalRows, fmt.Sprintf("%s%s%s[0m", color, strings.Repeat(" ", shortPad), shortMsg))
		if detail != "" {
			if len(detail) > a.width {
				detail = detail[:a.width]
			}
			detailPad := (a.width - len(detail)) / 2
			if detailPad < 0 {
				detailPad = 0
			}
			approvalRows = append(approvalRows, fmt.Sprintf("[2m%s%s[0m", strings.Repeat(" ", detailPad), detail))
		}
		approvalRows = append(approvalRows, sep)
		return approvalRows
	}

	rows := []string{sep}

	// Config editor mode replaces input area
	if a.cfgActive {
		rows = append(rows, a.buildConfigRows()...)
		rows = append(rows, sep)
		return rows
	}

	// Menu mode replaces input area
	if a.menuActive && len(a.menuLines) > 0 {
		w := a.width
		if a.menuHeader != "" {
			rows = append(rows, fmt.Sprintf("\033[1m%s\033[0m", truncateWithEllipsis(a.menuHeader, w)))
		}
		maxVisible := getTerminalHeight() * 60 / 100
		if maxVisible < 1 {
			maxVisible = 1
		}
		total := len(a.menuLines)
		end := a.menuScrollOffset + maxVisible
		if end > total {
			end = total
		}
		for i := a.menuScrollOffset; i < end; i++ {
			line := a.menuLines[i]
			if i == a.menuCursor {
				rows = append(rows, fmt.Sprintf("\033[36;1m%s ◆\033[0m", truncateWithEllipsis(line, w-2)))
			} else {
				rows = append(rows, truncateWithEllipsis(line, w))
			}
		}
		first := a.menuScrollOffset + 1
		last := end
		indicator := fmt.Sprintf("(%d->%d / %d)", first, last, total)
		rows = append(rows, fmt.Sprintf("\033[2m%s\033[0m", truncateWithEllipsis(indicator, w)))
		if a.menuModels != nil {
			hints := "←/→ sort column  Tab flip order  Enter select  Esc close"
			rows = append(rows, fmt.Sprintf("\033[2m%s\033[0m", truncateWithEllipsis(hints, w)))
		}
		rows = append(rows, sep)
		return rows
	}

	if a.promptLabel != "" {
		rows = append(rows, fmt.Sprintf("\033[33;1m%s\033[0m", a.promptLabel))
	}

	vlines := getVisualLines(a.input, a.cursor, a.width)
	for i, vl := range vlines {
		line := string(a.input[vl.start : vl.start+vl.length])
		if i == 0 {
			line = promptPrefix + line
		}
		rows = append(rows, line)
	}

	rows = append(rows, sep)

	// Ctrl+C / ESC hint (below separator, above status)
	if a.ctrlCHint {
		if a.agentRunning {
			rows = append(rows, fmt.Sprintf("\033[1;38;5;%dmPress Ctrl-C again to stop the agent\033[0m", 4))
		} else {
			rows = append(rows, fmt.Sprintf("\033[1;38;5;%dmPress Ctrl-C again to exit\033[0m", 4))
		}
	}
	if a.escHint {
		rows = append(rows, fmt.Sprintf("\033[1;38;5;%dmPress ESC again to stop the agent\033[0m", 4))
	}

	// Autocomplete (shown below input)
	hasAction := false
	if matches := a.autocompleteMatches(); len(matches) > 0 {
		hasAction = true
		for i, cmd := range matches {
			if i == a.autocompleteIdx {
				rows = append(rows, fmt.Sprintf("\033[36;1m%s ◆\033[0m", cmd))
			} else {
				rows = append(rows, cmd)
			}
		}
	}

	// Status indicators (only when no action is active)
	if !hasAction {
		// Line 1: branch: <name> -del/+add ↓behind↑ahead $cost [progress]
		branchLabel := ""
		branchTextWidth := 0
		if a.status.Branch != "" {
			branchLabel = "\033[2mbranch: " + a.status.Branch + "\033[0m"
			branchTextWidth = 8 + len(a.status.Branch) // "branch: " + name
		}
		diffLabel := ""
		diffTextWidth := 0
		if a.status.DiffDel > 0 || a.status.DiffAdd > 0 {
			delStr := fmt.Sprintf("-%d", a.status.DiffDel)
			addStr := fmt.Sprintf("+%d", a.status.DiffAdd)
			// red for deletions, green for additions, dim
			diffLabel = " \033[2;31m" + delStr + "\033[0m\033[2m/\033[0m\033[2;32m" + addStr + "\033[0m"
			diffTextWidth = 1 + len(delStr) + 1 + len(addStr) // space + del + "/" + add
		}
		commitLabel := ""
		commitTextWidth := 0
		if a.status.HasUpstream {
			commitStr := fmt.Sprintf(" ↓%d↑%d", a.status.Behind, a.status.Ahead)
			commitLabel = "\033[2m" + commitStr + "\033[0m"
			commitTextWidth = uniseg.StringWidth(commitStr)
		}
		costLabel := ""
		costTextWidth := 0
		if a.sessionCostUSD > 0 {
			costStr := formatCost(a.sessionCostUSD)
			costLabel = " \033[2m" + costStr + "\033[0m"
			costTextWidth = 1 + len(costStr)
		}
		contextTokens := a.lastInputTokens + len(a.input)/charsPerToken
		contextWindow := 200000
		if m := findModelByID(a.models, a.config.resolveActiveModel(a.models)); m != nil {
			contextWindow = m.ContextWindow
		}
		bar := progressBar(contextTokens, contextWindow)
		barWidth := 3
		padding := a.width - branchTextWidth - diffTextWidth - commitTextWidth - costTextWidth - barWidth - 1
		if padding < 0 {
			padding = 0
		}
		rows = append(rows, branchLabel+diffLabel+commitLabel+costLabel+strings.Repeat(" ", padding)+bar+" ")

		// Line 2: container status (always shown when we have status text)
		if a.containerStatusText != "" {
			style := "\033[2m" // dim
			if a.containerErr != nil {
				style = "\033[31m" // red
			}
			rows = append(rows, style+"container: "+a.containerStatusText+"\033[0m\033[K")
		}

		// Line 3: worktree: <name> (only when actually in a worktree)
		if a.status.WorktreeName != "" && a.isInWorktree() {
			rows = append(rows, "\033[2mworktree: "+a.status.WorktreeName+"\033[0m\033[K")
		}
	}

	return rows
}

func (a *App) positionCursor(buf *strings.Builder) {
	s := a.scrollShift
	if a.cfgActive {
		if a.cfgEditing {
			// Position cursor in the edit field: separator + tab bar (1) + cursor row
			fieldRow := a.sepRow + 1 + a.cfgCursor + 1 // +1 for tab bar row
			fields := a.cfgCurrentFields()
			col := 0
			if a.cfgCursor < len(fields) {
				col = len(fields[a.cfgCursor].label) + 2 // "label: "
			}
			col += a.cfgEditCursor
			buf.WriteString("\033[?25h")
			buf.WriteString(fmt.Sprintf("\033[%d;%dH", fieldRow-s, col+1))
		} else {
			buf.WriteString("\033[?25l")
			buf.WriteString(fmt.Sprintf("\033[%d;1H", a.sepRow+1-s))
		}
		return
	}
	if a.menuActive && len(a.menuLines) > 0 {
		// Menu between separators — hide cursor
		buf.WriteString("\033[?25l")
		buf.WriteString(fmt.Sprintf("\033[%d;1H", a.sepRow+1-s))
		return
	}
	if a.awaitingApproval {
		buf.WriteString("\033[?25l")
		buf.WriteString(fmt.Sprintf("\033[%d;1H", a.sepRow+1-s))
		return
	}
	buf.WriteString("\033[?25h")
	curLine, curCol := cursorVisualPos(a.input, a.cursor, a.width)
	buf.WriteString(fmt.Sprintf("\033[%d;%dH", a.inputStartRow+curLine-s, curCol+1))
}

func (a *App) render() {
	blockRows := a.buildBlockRows()

	a.sepRow = len(blockRows) + 1
	a.inputStartRow = a.sepRow + 1
	if a.promptLabel != "" {
		a.inputStartRow++
	}

	inputRows := a.buildInputRows()
	allRows := append(blockRows, inputRows...)
	totalRows := len(allRows)

	th := getTerminalHeight()
	newScrollShift := 0
	if totalRows > th {
		newScrollShift = totalRows - th
	}

	var buf strings.Builder

	if newScrollShift > 0 && a.scrollShift > 0 && newScrollShift >= a.scrollShift {
		// Content overflows and grew or stayed same: write only visible portion.
		// Scroll terminal down if content grew, then overwrite visible rows.
		if extra := newScrollShift - a.scrollShift; extra > 0 {
			buf.WriteString(fmt.Sprintf("\033[%d;1H", th))
			for i := 0; i < extra; i++ {
				buf.WriteString("\r\n")
			}
		}
		visibleRows := allRows[newScrollShift:]
		writeRows(&buf, visibleRows, 1)
	} else {
		// No overflow, or content shrank: write from top.
		if a.scrollShift > 0 {
			buf.WriteString("\033[H\033[2J\033[3J") // clear screen + scrollback
		}
		writeRows(&buf, allRows, 1)
	}

	buf.WriteString("\033[0m\033[J") // clear from cursor to end of screen

	a.prevRowCount = totalRows
	a.scrollShift = newScrollShift

	a.positionCursor(&buf)
	os.Stdout.WriteString(buf.String())
}

// renderFull clears the visible screen and scrollback, then does a full render.
// Use on resize (SIGWINCH) for an artifact-free re-render.
func (a *App) renderFull() {
	a.scrollShift = 0 // reset so render() writes from top
	os.Stdout.WriteString("\033[?25l\033[H\033[2J\033[3J") // hide cursor, clear screen + scrollback
	a.render() // render() → positionCursor() restores cursor visibility
}

func (a *App) renderInput() {
	inputRows := a.buildInputRows()
	totalRows := a.sepRow - 1 + len(inputRows)
	th := getTerminalHeight()

	newScrollShift := 0
	if totalRows > th {
		newScrollShift = totalRows - th
	}

	// If content shrank and we need to un-scroll, do a full render
	if newScrollShift < a.scrollShift {
		a.render()
		return
	}

	// Compute screen position of sepRow using current scroll state
	screenSepRow := a.sepRow - a.scrollShift
	if screenSepRow < 1 {
		a.render()
		return
	}

	var buf strings.Builder
	writeRows(&buf, inputRows, screenSepRow)
	buf.WriteString("\033[0m\033[J") // clear remaining lines

	a.scrollShift = newScrollShift
	a.prevRowCount = totalRows

	a.positionCursor(&buf)
	os.Stdout.WriteString(buf.String())
}

