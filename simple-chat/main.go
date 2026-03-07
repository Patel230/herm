package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type Block struct {
	Text string
}

var (
	blocks []Block
	width  int
	input  string
)

func getWidth() int {
	w, _, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return 80
	}
	return w
}

func render() {
	var buf strings.Builder

	// Home + clear entire screen + clear scrollback
	buf.WriteString("\033[H\033[2J\033[3J")

	// Blocks
	for _, b := range blocks {
		buf.WriteString(b.Text)
		buf.WriteString("\r\n")
	}

	// Separator (full width)
	buf.WriteString(strings.Repeat("─", width))
	buf.WriteString("\r\n")

	// Prompt
	buf.WriteString("❯ " + input)

	os.Stdout.WriteString(buf.String())
}

func main() {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Enter alternate screen, hide cursor during render
	fmt.Print("\033[?1049h")
	defer fmt.Print("\033[?1049l")

	width = getWidth()

	// Listen for window resize
	sigWinch := make(chan os.Signal, 1)
	signal.Notify(sigWinch, syscall.SIGWINCH)
	go func() {
		for range sigWinch {
			width = getWidth()
			render()
		}
	}()

	render()

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
		// Enter
		if buf[0] == '\r' {
			if input != "" {
				blocks = append(blocks, Block{Text: input})
				input = ""
			}
			render()
			continue
		}
		// Backspace
		if buf[0] == 127 {
			if len(input) > 0 {
				input = input[:len(input)-1]
				fmt.Print("\b \b")
			}
			continue
		}
		// Regular character
		input += string(buf[0])
		fmt.Printf("%c", buf[0])
	}
}
