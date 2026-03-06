package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestWorktreeListNavigation(t *testing.T) {
	items := []WorktreeInfo{
		{Path: "/wt/a", Branch: "branch-a", Clean: true, Active: false},
		{Path: "/wt/b", Branch: "branch-b", Clean: false, Active: true},
		{Path: "/wt/c", Branch: "branch-c", Clean: true, Active: false},
	}
	wl := newWorktreeList(items, "/wt/a", 80, 24)

	if wl.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", wl.cursor)
	}

	// Move down
	wl.HandleKey(EventKey{Key: KeyDown})
	if wl.cursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", wl.cursor)
	}

	// Move down again
	wl.HandleKey(EventKey{Key: KeyDown})
	if wl.cursor != 2 {
		t.Errorf("after second down: cursor = %d, want 2", wl.cursor)
	}

	// Down at bottom stays at bottom
	wl.HandleKey(EventKey{Key: KeyDown})
	if wl.cursor != 2 {
		t.Errorf("down at bottom: cursor = %d, want 2", wl.cursor)
	}

	// Move up
	wl.HandleKey(EventKey{Key: KeyUp})
	if wl.cursor != 1 {
		t.Errorf("after up: cursor = %d, want 1", wl.cursor)
	}

	// j/k navigation
	wl.HandleKey(EventKey{Key: KeyRune, Rune: 'k'})
	if wl.cursor != 0 {
		t.Errorf("after k: cursor = %d, want 0", wl.cursor)
	}

	wl.HandleKey(EventKey{Key: KeyRune, Rune: 'j'})
	if wl.cursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", wl.cursor)
	}
}

func TestWorktreeListEscReturnsToChat(t *testing.T) {
	a := newTestApp(80, 24)
	a.mode = modeWorktrees
	a.worktreeListC = newWorktreeList([]WorktreeInfo{
		{Path: "/wt/a", Branch: "main", Clean: true},
	}, "", 80, 24)

	simKey(a, KeyEscape)

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat (%d)", a.mode, modeChat)
	}
}

func TestWorktreeListCurrentMarked(t *testing.T) {
	items := []WorktreeInfo{
		{Path: "/wt/a", Branch: "branch-a", Clean: true},
		{Path: "/wt/b", Branch: "branch-b", Clean: true},
	}
	wl := newWorktreeList(items, "/wt/a", 80, 24)
	view := wl.View()

	if !strings.Contains(view, "●") {
		t.Error("view should contain '●' marker for the current worktree")
	}
}

func TestWorktreeListCurrentSessionMarker(t *testing.T) {
	items := []WorktreeInfo{
		{Path: "/wt/current", Branch: "current-branch", Clean: true},
		{Path: "/wt/other", Branch: "other-branch", Clean: false},
	}
	wl := newWorktreeList(items, "/wt/current", 80, 24)
	view := wl.View()

	// Current session worktree should show green dot marker
	if !strings.Contains(view, "●") {
		t.Error("view should contain ● for current session worktree")
	}
}

func TestWorktreeListNoActiveLabel(t *testing.T) {
	items := []WorktreeInfo{
		{Path: "/wt/a", Branch: "branch-a", Clean: true, Active: true},
		{Path: "/wt/b", Branch: "branch-b", Clean: true, Active: false},
	}
	wl := newWorktreeList(items, "", 80, 24)
	view := wl.View()

	if strings.Contains(view, "[active]") {
		t.Error("view should not contain [active] label")
	}
}

func TestWorktreeListEmpty(t *testing.T) {
	wl := newWorktreeList(nil, "", 80, 24)
	view := wl.View()

	if !strings.Contains(view, "No worktrees found") {
		t.Error("empty list should show 'No worktrees found'")
	}
}

func TestWorktreeListMsgPopulatesList(t *testing.T) {
	a := newTestApp(80, 24)
	a.mode = modeWorktrees
	a.worktreePath = "/wt/current"

	items := []WorktreeInfo{
		{Path: "/wt/current", Branch: "main", Clean: true, Active: true},
		{Path: "/wt/other", Branch: "feature", Clean: false, Active: false},
	}
	simResult(a, worktreeListMsg{items: items})

	if len(a.worktreeListC.items) != 2 {
		t.Errorf("worktreeList items = %d, want 2", len(a.worktreeListC.items))
	}
	if a.worktreeListC.currentPath != "/wt/current" {
		t.Errorf("currentPath = %q, want %q", a.worktreeListC.currentPath, "/wt/current")
	}
}

func TestWorktreeListMsgError(t *testing.T) {
	a := newTestApp(80, 24)
	a.mode = modeWorktrees

	simResult(a, worktreeListMsg{err: fmt.Errorf("not in git repo")})

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat after error", a.mode)
	}
	if len(a.messages) == 0 {
		t.Fatal("should have error message")
	}
	last := a.messages[len(a.messages)-1]
	if last.kind != msgError {
		t.Errorf("last message kind = %d, want msgError", last.kind)
	}
}

func TestWorktreeCommandAutocomplete(t *testing.T) {
	matches := filterCommands("/wor")
	if len(matches) != 1 || matches[0] != "/worktrees" {
		t.Errorf("filterCommands('/wor') = %v, want [/worktrees]", matches)
	}
}
