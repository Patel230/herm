// debuglog.go implements file-based conversation debug logging.
// When debug mode is enabled, every conversation gets a debug file in
// .herm/debug/ that logs system prompts, tool calls/results, agent events,
// usage stats, user messages, and session summary.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const debugDir = "debug"

// initDebugLog creates the .herm/debug/ directory under repoRoot and opens a
// new debug log file named debug-<timestamp>.log. Returns the open file, the
// file path, and any error.
func initDebugLog(repoRoot string) (*os.File, string, error) {
	dir := filepath.Join(repoRoot, configDir, debugDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", fmt.Errorf("creating debug dir: %w", err)
	}

	name := fmt.Sprintf("debug-%s.log", time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, "", fmt.Errorf("opening debug file: %w", err)
	}
	return f, path, nil
}

// debugWrite appends a delimited section to the debug file.
// If f is nil, it's a no-op.
func debugWrite(f *os.File, section string, content string) {
	if f == nil {
		return
	}
	fmt.Fprintf(f, "\n── %s ──\n%s\n", section, content)
}

// closeDebugLog closes the debug file if it's open.
func closeDebugLog(f *os.File) {
	if f != nil {
		f.Close()
	}
}

// debugActive returns true if debug mode is enabled via config or CLI flag.
func (a *App) debugActive() bool {
	return a.config.DebugMode || a.cliDebug
}

// debugWriteSection is a convenience method that writes to the app's debug file.
func (a *App) debugWriteSection(section, content string) {
	debugWrite(a.debugFile, section, content)
}

// initAppDebugLog initializes the debug log file for the app if debug mode is active.
// Should be called after repoRoot is known (i.e. after workspaceMsg).
func (a *App) initAppDebugLog() {
	if !a.debugActive() {
		return
	}
	root := a.repoRoot
	if root == "" {
		// Fallback to current directory if not in a git repo.
		root, _ = os.Getwd()
	}
	f, path, err := initDebugLog(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "debug: failed to create debug log: %v\n", err)
		return
	}
	a.debugFile = f
	a.debugFilePath = path
}
