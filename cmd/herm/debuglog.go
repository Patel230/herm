// debuglog.go implements JSON trace debug logging for sessions.
// When debug mode is enabled, every conversation gets a .json trace file in
// .herm/debug/ that captures LLM calls, tool calls, sub-agent traces, and
// session totals.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const debugDir = "debug"

// debugActive returns true if debug mode is enabled via config or CLI flag.
func (a *App) debugActive() bool {
	return a.config.DebugMode || a.cliDebug
}

// debugWriteSection is a legacy no-op kept during migration.
// All callers will be replaced with trace collector methods.
func (a *App) debugWriteSection(section, content string) {}

// initAppDebugLog initializes the JSON trace collector for the app if debug mode is active.
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
	dir := filepath.Join(root, configDir, debugDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "debug: failed to create debug dir: %v\n", err)
		return
	}
	name := fmt.Sprintf("debug-%s.json", time.Now().Format("20060102-150405"))
	a.traceFilePath = filepath.Join(dir, name)
	a.traceCollector = NewTraceCollector(a.sessionID, a.models)
	a.traceCollector.SetGitInfo(a.status.Branch, a.repoRoot)
}

// debugWriteSessionSummary is a legacy no-op kept during migration.
// The trace collector's info object replaces this.
func (a *App) debugWriteSessionSummary() {}

// regenerateDebugFile is a legacy no-op kept during migration.
// JSON trace files are written on events, not on display changes.
func (a *App) regenerateDebugFile() {}
