package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestBranchListNavigation(t *testing.T) {
	branches := []string{"main", "feature-a", "feature-b"}
	bl := newBranchList(branches, "main", 80, 24)

	if bl.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", bl.cursor)
	}

	// Move down
	bl.HandleKey(EventKey{Key: KeyDown})
	if bl.cursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", bl.cursor)
	}

	// Move down again
	bl.HandleKey(EventKey{Key: KeyDown})
	if bl.cursor != 2 {
		t.Errorf("after second down: cursor = %d, want 2", bl.cursor)
	}

	// Down at bottom stays at bottom
	bl.HandleKey(EventKey{Key: KeyDown})
	if bl.cursor != 2 {
		t.Errorf("down at bottom: cursor = %d, want 2", bl.cursor)
	}

	// Move up
	bl.HandleKey(EventKey{Key: KeyUp})
	if bl.cursor != 1 {
		t.Errorf("after up: cursor = %d, want 1", bl.cursor)
	}
}

func TestBranchListFilterNarrowsResults(t *testing.T) {
	branches := []string{"main", "feature-a", "feature-b", "bugfix-1"}
	bl := newBranchList(branches, "main", 80, 24)

	if len(bl.filtered) != 4 {
		t.Fatalf("initial filtered = %d, want 4", len(bl.filtered))
	}

	// Type "feature" into the filter input
	for _, r := range "feature" {
		bl.HandleKey(EventKey{Key: KeyRune, Rune: r})
	}

	if len(bl.filtered) != 2 {
		t.Errorf("filtered after 'feature' = %d, want 2", len(bl.filtered))
	}
	for _, b := range bl.filtered {
		if !strings.Contains(b, "feature") {
			t.Errorf("filtered branch %q should contain 'feature'", b)
		}
	}

	// Cursor should reset to 0 after filter
	if bl.cursor != 0 {
		t.Errorf("cursor after filter = %d, want 0", bl.cursor)
	}
}

func TestBranchListFilterCaseInsensitive(t *testing.T) {
	branches := []string{"main", "Feature-A", "FEATURE-B"}
	bl := newBranchList(branches, "main", 80, 24)

	for _, r := range "feature" {
		bl.HandleKey(EventKey{Key: KeyRune, Rune: r})
	}

	if len(bl.filtered) != 2 {
		t.Errorf("filtered = %d, want 2 (case-insensitive)", len(bl.filtered))
	}
}

func TestBranchListEnterTriggersSelection(t *testing.T) {
	branches := []string{"main", "feature-a"}
	bl := newBranchList(branches, "main", 80, 24)

	// Move to feature-a
	bl.HandleKey(EventKey{Key: KeyDown})

	// Press Enter
	bl.HandleKey(EventKey{Key: KeyEnter})

	if bl.selected != "feature-a" {
		t.Errorf("selected = %q, want %q", bl.selected, "feature-a")
	}
}

func TestBranchListEnterOnEmptyFilteredNoop(t *testing.T) {
	branches := []string{"main"}
	bl := newBranchList(branches, "main", 80, 24)

	// Type something that matches nothing
	for _, r := range "zzz" {
		bl.HandleKey(EventKey{Key: KeyRune, Rune: r})
	}

	if len(bl.filtered) != 0 {
		t.Fatalf("filtered should be empty, got %d", len(bl.filtered))
	}

	bl.HandleKey(EventKey{Key: KeyEnter})
	if bl.selected != "" {
		t.Errorf("selected should be empty on empty list, got %q", bl.selected)
	}
}

func TestBranchListEscReturnsToChat(t *testing.T) {
	a := newTestApp(80, 24)
	a.mode = modeBranches
	a.branchListC = newBranchList([]string{"main", "feature"}, "main", 80, 24)

	simKey(a, KeyEscape)

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat (%d)", a.mode, modeChat)
	}
}

func TestBranchListCurrentMarked(t *testing.T) {
	branches := []string{"main", "feature-a"}
	bl := newBranchList(branches, "main", 80, 24)
	view := bl.View()

	if !strings.Contains(view, "●") {
		t.Error("view should contain '●' marker for the current branch")
	}
}

func TestBranchListEmpty(t *testing.T) {
	bl := newBranchList(nil, "", 80, 24)
	view := bl.View()

	if !strings.Contains(view, "No branches found") {
		t.Error("empty list should show 'No branches found'")
	}
}

func TestBranchListNoMatchingFilter(t *testing.T) {
	branches := []string{"main", "feature"}
	bl := newBranchList(branches, "main", 80, 24)

	for _, r := range "zzz" {
		bl.HandleKey(EventKey{Key: KeyRune, Rune: r})
	}

	view := bl.View()
	if !strings.Contains(view, "No matching branches") {
		t.Error("view should show 'No matching branches' when filter matches nothing")
	}
}

func TestBranchListMsgPopulatesList(t *testing.T) {
	a := newTestApp(80, 24)
	a.mode = modeBranches

	simResult(a, branchListMsg{
		items:         []string{"main", "feature-a", "feature-b"},
		currentBranch: "main",
	})

	if len(a.branchListC.items) != 3 {
		t.Errorf("branchList items = %d, want 3", len(a.branchListC.items))
	}
	if a.branchListC.currentBranch != "main" {
		t.Errorf("currentBranch = %q, want %q", a.branchListC.currentBranch, "main")
	}
}

func TestBranchListMsgError(t *testing.T) {
	a := newTestApp(80, 24)
	a.mode = modeBranches

	simResult(a, branchListMsg{err: fmt.Errorf("not in git repo")})

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

func TestBranchCheckoutMsgUpdatesStatusBar(t *testing.T) {
	a := newTestApp(80, 24)
	a.mode = modeBranches
	a.status.Branch = "main"

	simResult(a, branchCheckoutMsg{branch: "feature-a"})

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat after checkout", a.mode)
	}
	if a.status.Branch != "feature-a" {
		t.Errorf("status.Branch = %q, want %q", a.status.Branch, "feature-a")
	}
	if len(a.messages) == 0 {
		t.Fatal("should have success message")
	}
	last := a.messages[len(a.messages)-1]
	if last.kind != msgSuccess {
		t.Errorf("last message kind = %d, want msgSuccess", last.kind)
	}
}

func TestBranchCheckoutMsgError(t *testing.T) {
	a := newTestApp(80, 24)
	a.mode = modeBranches
	a.status.Branch = "main"

	simResult(a, branchCheckoutMsg{
		branch: "nonexistent",
		err:    fmt.Errorf("branch not found"),
	})

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat after error", a.mode)
	}
	// Branch should NOT change on error
	if a.status.Branch != "main" {
		t.Errorf("status.Branch = %q, want %q (unchanged)", a.status.Branch, "main")
	}
	if len(a.messages) == 0 {
		t.Fatal("should have error message")
	}
	last := a.messages[len(a.messages)-1]
	if last.kind != msgError {
		t.Errorf("last message kind = %d, want msgError", last.kind)
	}
}

func TestBranchCommandAutocomplete(t *testing.T) {
	matches := filterCommands("/bra")
	if len(matches) != 1 || matches[0] != "/branches" {
		t.Errorf("filterCommands('/bra') = %v, want [/branches]", matches)
	}
}
