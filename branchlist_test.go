package main

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestBranchListNavigation(t *testing.T) {
	branches := []string{"main", "feature-a", "feature-b"}
	bl := newBranchList(branches, "main", 80, 24)

	if bl.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", bl.cursor)
	}

	// Move down
	bl, _ = bl.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if bl.cursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", bl.cursor)
	}

	// Move down again
	bl, _ = bl.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if bl.cursor != 2 {
		t.Errorf("after second down: cursor = %d, want 2", bl.cursor)
	}

	// Down at bottom stays at bottom
	bl, _ = bl.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if bl.cursor != 2 {
		t.Errorf("down at bottom: cursor = %d, want 2", bl.cursor)
	}

	// Move up
	bl, _ = bl.Update(tea.KeyPressMsg{Code: tea.KeyUp})
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
		bl, _ = bl.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
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
		bl, _ = bl.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	if len(bl.filtered) != 2 {
		t.Errorf("filtered = %d, want 2 (case-insensitive)", len(bl.filtered))
	}
}

func TestBranchListEnterTriggersSelection(t *testing.T) {
	branches := []string{"main", "feature-a"}
	bl := newBranchList(branches, "main", 80, 24)

	// Move to feature-a
	bl, _ = bl.Update(tea.KeyPressMsg{Code: tea.KeyDown})

	// Press Enter
	var cmd tea.Cmd
	bl, cmd = bl.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("enter should return a command")
	}

	msg := cmd()
	sel, ok := msg.(branchSelected)
	if !ok {
		t.Fatalf("command should produce branchSelected, got %T", msg)
	}
	if sel.name != "feature-a" {
		t.Errorf("selected branch = %q, want %q", sel.name, "feature-a")
	}
}

func TestBranchListEnterOnEmptyFilteredNoop(t *testing.T) {
	branches := []string{"main"}
	bl := newBranchList(branches, "main", 80, 24)

	// Type something that matches nothing
	for _, r := range "zzz" {
		bl, _ = bl.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	if len(bl.filtered) != 0 {
		t.Fatalf("filtered should be empty, got %d", len(bl.filtered))
	}

	var cmd tea.Cmd
	bl, cmd = bl.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Error("enter on empty filtered list should return nil cmd")
	}
}

func TestBranchListEscReturnsToChat(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	m.mode = modeBranches
	m.branchListC = newBranchList([]string{"main", "feature"}, "main", 80, 24)

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = result.(model)

	if m.mode != modeChat {
		t.Errorf("mode = %d, want modeChat (%d)", m.mode, modeChat)
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
		bl, _ = bl.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	view := bl.View()
	if !strings.Contains(view, "No matching branches") {
		t.Error("view should show 'No matching branches' when filter matches nothing")
	}
}

func TestBranchListMsgPopulatesList(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)
	m.mode = modeBranches

	result, _ := m.Update(branchListMsg{
		items:         []string{"main", "feature-a", "feature-b"},
		currentBranch: "main",
	})
	m = result.(model)

	if len(m.branchListC.items) != 3 {
		t.Errorf("branchList items = %d, want 3", len(m.branchListC.items))
	}
	if m.branchListC.currentBranch != "main" {
		t.Errorf("currentBranch = %q, want %q", m.branchListC.currentBranch, "main")
	}
}

func TestBranchListMsgError(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)
	m.mode = modeBranches

	result, _ := m.Update(branchListMsg{err: fmt.Errorf("not in git repo")})
	m = result.(model)

	if m.mode != modeChat {
		t.Errorf("mode = %d, want modeChat after error", m.mode)
	}
	if len(m.messages) == 0 {
		t.Fatal("should have error message")
	}
	last := m.messages[len(m.messages)-1]
	if last.kind != msgError {
		t.Errorf("last message kind = %d, want msgError", last.kind)
	}
}

func TestBranchCheckoutMsgUpdatesStatusBar(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)
	m.mode = modeBranches
	m.status.Branch = "main"

	result, _ := m.Update(branchCheckoutMsg{branch: "feature-a"})
	m = result.(model)

	if m.mode != modeChat {
		t.Errorf("mode = %d, want modeChat after checkout", m.mode)
	}
	if m.status.Branch != "feature-a" {
		t.Errorf("status.Branch = %q, want %q", m.status.Branch, "feature-a")
	}
	if len(m.messages) == 0 {
		t.Fatal("should have success message")
	}
	last := m.messages[len(m.messages)-1]
	if last.kind != msgSuccess {
		t.Errorf("last message kind = %d, want msgSuccess", last.kind)
	}
}

func TestBranchCheckoutMsgError(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)
	m.mode = modeBranches
	m.status.Branch = "main"

	result, _ := m.Update(branchCheckoutMsg{
		branch: "nonexistent",
		err:    fmt.Errorf("branch not found"),
	})
	m = result.(model)

	if m.mode != modeChat {
		t.Errorf("mode = %d, want modeChat after error", m.mode)
	}
	// Branch should NOT change on error
	if m.status.Branch != "main" {
		t.Errorf("status.Branch = %q, want %q (unchanged)", m.status.Branch, "main")
	}
	if len(m.messages) == 0 {
		t.Fatal("should have error message")
	}
	last := m.messages[len(m.messages)-1]
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
