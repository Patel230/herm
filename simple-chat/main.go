package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"unicode/utf8"

	"golang.org/x/term"
)

type Block struct {
	Text string
}

const (
	promptPrefix     = "❯ "
	promptPrefixCols = 2 // display width of "❯ "
)

var (
	blocks []Block
	width  int
	input  string
	sepRow int // 1-based row where the top separator lives
)

func getWidth() int {
	w, _, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return 80
	}
	return w
}

func lastInputLine() string {
	if idx := strings.LastIndex(input, "\n"); idx >= 0 {
		return input[idx+1:]
	}
	return input
}

func cursorCol() int {
	col := utf8.RuneCountInString(lastInputLine()) + 1
	if !strings.Contains(input, "\n") {
		col += promptPrefixCols
	}
	return col
}

// writeInputArea writes the top separator, prompt+input, and bottom separator.
// It then positions the cursor at the end of the input text.
func writeInputArea(buf *strings.Builder) {
	buf.WriteString(strings.Repeat("─", width))
	buf.WriteString("\r\n")

	displayInput := strings.ReplaceAll(input, "\n", "\r\n")
	buf.WriteString(promptPrefix + displayInput)
	buf.WriteString("\r\n")

	buf.WriteString(strings.Repeat("─", width))

	// Move cursor back up to the last input line (1 up from bottom separator)
	buf.WriteString(fmt.Sprintf("\033[A\033[%dG", cursorCol()))
}

// renderAll clears the entire screen and redraws everything.
func renderAll() {
	var buf strings.Builder
	buf.WriteString("\033[H\033[2J\033[3J")

	row := 1
	for _, b := range blocks {
		lines := strings.Split(b.Text, "\n")
		for _, l := range lines {
			buf.WriteString(l)
			buf.WriteString("\r\n")
			row++
		}
		// empty line after each block
		buf.WriteString("\r\n")
		row++
	}

	sepRow = row
	writeInputArea(&buf)
	os.Stdout.WriteString(buf.String())
}

// renderInput redraws only from the top separator down (no full screen clear).
func renderInput() {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("\033[%d;1H\033[J", sepRow))
	writeInputArea(&buf)
	os.Stdout.WriteString(buf.String())
}

func main() {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Enter alternate screen
	fmt.Print("\033[?1049h")
	defer fmt.Print("\033[?1049l")

	width = getWidth()

	// Listen for window resize
	sigWinch := make(chan os.Signal, 1)
	signal.Notify(sigWinch, syscall.SIGWINCH)
	go func() {
		for range sigWinch {
			width = getWidth()
			renderAll()
		}
	}()

	renderAll()

	buf := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}
		// Ctrl-C or Ctrl-D to quit
		if buf[0] == 3 || buf[0] == 4 {
			break
		}
		// Shift+Enter (newline)
		if buf[0] == '\n' {
			input += "\n"
			renderInput()
			continue
		}
		// Enter — submit
		if buf[0] == '\r' {
			if input != "" {
				blocks = append(blocks, Block{Text: input})
				input = ""
			}
			renderAll()
			continue
		}
		// Backspace
		if buf[0] == 127 {
			if len(input) > 0 {
				removed := input[len(input)-1]
				input = input[:len(input)-1]
				if removed == '\n' {
					// Line count changed — re-render input area
					renderInput()
				} else {
					// Same line — just erase one character
					os.Stdout.WriteString("\b \b")
				}
			}
			continue
		}
		// Regular character — append and echo directly (no re-render)
		input += string(buf[0])
		os.Stdout.WriteString(string(buf[0]))
	}
}
