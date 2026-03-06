package main

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

// termState holds the terminal raw mode state for cleanup.
type termState struct {
	fd       int
	oldState *term.State
}

// enterRawMode puts the terminal into raw mode and returns state for restoration.
func enterRawMode() (*termState, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	return &termState{fd: fd, oldState: oldState}, nil
}

// restoreTerminal restores the terminal to its previous state.
func restoreTerminal(ts *termState) {
	if ts != nil && ts.oldState != nil {
		term.Restore(ts.fd, ts.oldState)
	}
}

// getTerminalSize returns the current terminal width and height.
func getTerminalSize() (width, height int, err error) {
	return term.GetSize(int(os.Stdout.Fd()))
}

// watchResize listens for SIGWINCH signals and sends resize events to the channel.
// It runs until the stop channel is closed.
func watchResize(events chan<- EventResize, stop <-chan struct{}) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-sigCh:
			w, h, err := getTerminalSize()
			if err == nil {
				events <- EventResize{Width: w, Height: h}
			}
		case <-stop:
			return
		}
	}
}
