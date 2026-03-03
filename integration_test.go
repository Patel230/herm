package main

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestFullFlowPasteAndConfigThreshold exercises the end-to-end flow:
// start app → paste long text → verify collapsed display → send →
// /config → change threshold → verify new threshold applies.
func TestFullFlowPasteAndConfigThreshold(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	// --- Step 1: Paste long text above default threshold (200 chars) ---
	longText := strings.Repeat("X", 300)
	m = paste(m, longText)

	// Textarea should show the placeholder, not the raw text
	if !strings.Contains(m.textarea.Value(), "[pasted #1 | 300 chars]") {
		t.Fatalf("textarea should contain paste placeholder, got %q", m.textarea.Value())
	}
	if m.pasteCount != 1 {
		t.Fatalf("pasteCount = %d, want 1", m.pasteCount)
	}

	// --- Step 2: Send the pasted message ---
	m = sendKey(m, tea.KeyEnter)

	if len(m.messages) != 1 {
		t.Fatalf("messages count = %d, want 1", len(m.messages))
	}
	// Sent message should have expanded content, not the placeholder
	if m.messages[0].content != longText {
		t.Errorf("sent message should be expanded paste content")
	}

	// Viewport should show the expanded content
	v := m.View()
	if strings.Contains(v.Content, "[pasted #1") {
		t.Error("viewport should not show paste placeholder after send")
	}
	if !strings.Contains(v.Content, "XXXXXXX") {
		t.Error("viewport should show expanded paste content")
	}

	// --- Step 3: Open /config and change threshold ---
	m = typeString(m, "/config")
	m = sendKey(m, tea.KeyEnter)

	if m.mode != modeConfig {
		t.Fatalf("mode = %d, want modeConfig", m.mode)
	}

	// Change threshold to 500
	m.configForm.fields[0].input.SetValue("500")

	// Save config
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(model)

	if m.mode != modeChat {
		t.Fatalf("mode = %d, want modeChat after saving config", m.mode)
	}
	if m.config.PasteCollapseMinChars != 500 {
		t.Fatalf("threshold = %d, want 500", m.config.PasteCollapseMinChars)
	}

	// --- Step 4: Paste text between old and new threshold ---
	// 300 chars is above old threshold (200) but below new threshold (500)
	mediumText := strings.Repeat("Y", 300)
	m = paste(m, mediumText)

	// Should NOT be collapsed — it's below the new threshold of 500
	if m.textarea.Value() != mediumText {
		t.Errorf("paste below new threshold should be verbatim, got %q", m.textarea.Value())
	}
	if m.pasteCount != 1 {
		t.Errorf("pasteCount should still be 1, got %d", m.pasteCount)
	}

	// --- Step 5: Paste text above the new threshold ---
	m.textarea.Reset()
	hugeText := strings.Repeat("Z", 600)
	m = paste(m, hugeText)

	if !strings.Contains(m.textarea.Value(), "[pasted #2 | 600 chars]") {
		t.Errorf("paste above new threshold should be collapsed, got %q", m.textarea.Value())
	}
	if m.pasteCount != 2 {
		t.Errorf("pasteCount = %d, want 2", m.pasteCount)
	}
}

// TestFullFlowConfigDiscardPreservesThreshold verifies that Esc in /config
// does not change the active threshold.
func TestFullFlowConfigDiscardPreservesThreshold(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	originalThreshold := m.config.PasteCollapseMinChars

	// Open /config
	m = typeString(m, "/config")
	m = sendKey(m, tea.KeyEnter)

	// Change value but discard
	m.configForm.fields[0].input.SetValue("999")
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = result.(model)

	if m.config.PasteCollapseMinChars != originalThreshold {
		t.Errorf("threshold = %d, want %d (should be unchanged after discard)",
			m.config.PasteCollapseMinChars, originalThreshold)
	}

	// Paste at original threshold should still collapse
	longText := strings.Repeat("x", originalThreshold)
	m = paste(m, longText)
	if !strings.Contains(m.textarea.Value(), "[pasted #1") {
		t.Error("paste at original threshold should still collapse after config discard")
	}
}

// TestFullFlowMultiplePastesThenMessages exercises sending a mix of
// normal messages and paste messages, verifying the message feed integrity.
func TestFullFlowMultiplePastesThenMessages(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	// Send a normal message
	m = typeString(m, "hello everyone")
	m = sendKey(m, tea.KeyEnter)

	// Paste a long message
	paste1 := strings.Repeat("A", 300)
	m = paste(m, paste1)
	m = sendKey(m, tea.KeyEnter)

	// Send another normal message
	m = typeString(m, "that was a big paste")
	m = sendKey(m, tea.KeyEnter)

	// Paste another long message
	paste2 := strings.Repeat("B", 500)
	m = paste(m, paste2)
	m = sendKey(m, tea.KeyEnter)

	// Verify all messages present and in order
	if len(m.messages) != 4 {
		t.Fatalf("messages count = %d, want 4", len(m.messages))
	}
	if m.messages[0].content != "hello everyone" {
		t.Errorf("messages[0] = %q, want %q", m.messages[0].content, "hello everyone")
	}
	if m.messages[1].content != paste1 {
		t.Error("messages[1] should contain expanded paste #1")
	}
	if m.messages[2].content != "that was a big paste" {
		t.Errorf("messages[2] = %q, want %q", m.messages[2].content, "that was a big paste")
	}
	if m.messages[3].content != paste2 {
		t.Error("messages[3] should contain expanded paste #2")
	}

	// Viewport should show all expanded content
	v := m.View()
	if !strings.Contains(v.Content, "hello everyone") {
		t.Error("viewport missing normal message 1")
	}
	if !strings.Contains(v.Content, "AAAA") {
		t.Error("viewport missing expanded paste 1")
	}
	if !strings.Contains(v.Content, "that was a big paste") {
		t.Error("viewport missing normal message 2")
	}
	if !strings.Contains(v.Content, "BBBB") {
		t.Error("viewport missing expanded paste 2")
	}
}

// TestFullFlowUnknownCommandThenValidCommand verifies error recovery:
// unknown command shows error, then /config works normally.
func TestFullFlowUnknownCommandThenValidCommand(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	// Try an unknown command
	m = typeString(m, "/help")
	m = sendKey(m, tea.KeyEnter)

	if m.mode != modeChat {
		t.Error("should stay in chat mode after unknown command")
	}
	if len(m.messages) != 1 || !m.messages[0].isSystem {
		t.Fatal("should have one system error message")
	}
	if !strings.Contains(m.messages[0].content, "/help") {
		t.Errorf("error should mention /help, got %q", m.messages[0].content)
	}

	// Now use /config — should work fine
	m = typeString(m, "/config")
	m = sendKey(m, tea.KeyEnter)

	if m.mode != modeConfig {
		t.Error("should enter config mode after /config")
	}

	// Cancel and return to chat
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = result.(model)

	if m.mode != modeChat {
		t.Error("should return to chat mode after Esc")
	}
	// Should have the error message + the discard message
	if len(m.messages) != 2 {
		t.Errorf("messages count = %d, want 2", len(m.messages))
	}
}

// TestFullFlowConfigSaveAndVerifyWithPaste verifies the complete cycle:
// change config → save → paste at boundary → verify threshold applies.
func TestFullFlowConfigSaveAndVerifyWithPaste(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	// Lower threshold to 50
	m = typeString(m, "/config")
	m = sendKey(m, tea.KeyEnter)
	m.configForm.fields[0].input.SetValue("50")
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(model)

	if m.config.PasteCollapseMinChars != 50 {
		t.Fatalf("threshold = %d, want 50", m.config.PasteCollapseMinChars)
	}

	// Paste exactly at threshold (50 chars) — should collapse
	exactText := strings.Repeat("Q", 50)
	m = paste(m, exactText)
	if !strings.Contains(m.textarea.Value(), "[pasted #1 | 50 chars]") {
		t.Errorf("paste at exact threshold should collapse, got %q", m.textarea.Value())
	}
	m = sendKey(m, tea.KeyEnter)

	// Paste below threshold (49 chars) — should be verbatim
	belowText := strings.Repeat("R", 49)
	m = paste(m, belowText)
	if m.textarea.Value() != belowText {
		t.Errorf("paste below threshold should be verbatim, got %q", m.textarea.Value())
	}
	m = sendKey(m, tea.KeyEnter)

	// Verify messages
	// messages[0] = "Config saved." (system)
	// messages[1] = expanded paste (50 Q's)
	// messages[2] = verbatim paste (49 R's)
	if len(m.messages) != 3 {
		t.Fatalf("messages count = %d, want 3", len(m.messages))
	}
	if m.messages[1].content != exactText {
		t.Error("message[1] should contain expanded paste content")
	}
	if m.messages[2].content != belowText {
		t.Error("message[2] should contain verbatim paste content")
	}
}

// TestFullFlowConfigValidationThenSave tests that invalid config input
// is rejected, then corrected input saves successfully.
func TestFullFlowConfigValidationThenSave(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	m = typeString(m, "/config")
	m = sendKey(m, tea.KeyEnter)

	// Try invalid value
	m.configForm.fields[0].input.SetValue("not-a-number")
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(model)

	// Should stay in config mode
	if m.mode != modeConfig {
		t.Fatal("should stay in config mode with invalid input")
	}
	if m.configForm.fields[0].err == "" {
		t.Error("should show validation error")
	}

	// Fix the value and save
	m.configForm.fields[0].input.SetValue("100")
	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(model)

	if m.mode != modeChat {
		t.Fatal("should return to chat mode after valid save")
	}
	if m.config.PasteCollapseMinChars != 100 {
		t.Errorf("threshold = %d, want 100", m.config.PasteCollapseMinChars)
	}
}

// TestFullFlowPasteWithSurroundingText tests paste collapsing when
// the user types text before and after a paste, then sends.
func TestFullFlowPasteWithSurroundingText(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	// Type prefix, paste, type suffix, send
	m = typeString(m, "see: ")
	longText := strings.Repeat("C", 250)
	m = paste(m, longText)
	m = typeString(m, " done")
	m = sendKey(m, tea.KeyEnter)

	if len(m.messages) != 1 {
		t.Fatalf("messages count = %d, want 1", len(m.messages))
	}

	content := m.messages[0].content
	if !strings.HasPrefix(content, "see: ") {
		t.Errorf("message should start with 'see: ', got %q", content[:20])
	}
	if !strings.Contains(content, longText) {
		t.Error("message should contain the expanded paste content")
	}
	if !strings.HasSuffix(content, " done") {
		t.Errorf("message should end with ' done', got suffix %q", content[len(content)-10:])
	}
}

// TestFullFlowResizeDuringPasteAndConfig tests window resizing during
// paste collapsing and config mode doesn't break the UI.
func TestFullFlowResizeDuringPasteAndConfig(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	// Paste and resize before sending
	longText := strings.Repeat("D", 300)
	m = paste(m, longText)
	m = resize(m, 120, 40)

	// Textarea should still have the placeholder
	if !strings.Contains(m.textarea.Value(), "[pasted #1") {
		t.Error("resize should not lose paste placeholder")
	}

	m = sendKey(m, tea.KeyEnter)

	// Viewport at new size should show expanded content
	v := m.View()
	if !strings.Contains(v.Content, "DDDDD") {
		t.Error("viewport should show expanded paste after resize")
	}

	// Enter config mode and resize
	m = typeString(m, "/config")
	m = sendKey(m, tea.KeyEnter)

	m = resize(m, 60, 20)
	if m.mode != modeConfig {
		t.Error("resize should not exit config mode")
	}
	if m.width != 60 || m.height != 20 {
		t.Errorf("dimensions = %dx%d, want 60x20", m.width, m.height)
	}

	// View in config mode should not panic
	v = m.View()
	if !strings.Contains(v.Content, "Configuration") {
		t.Error("config view should render after resize")
	}
}

// TestFullFlowPasteCounterPersistsAcrossConfigVisits verifies the paste
// counter doesn't reset when entering and exiting config mode.
func TestFullFlowPasteCounterPersistsAcrossConfigVisits(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	// Paste twice
	longText := strings.Repeat("E", 300)
	m = paste(m, longText)
	m = sendKey(m, tea.KeyEnter)
	m = paste(m, longText)
	m = sendKey(m, tea.KeyEnter)

	if m.pasteCount != 2 {
		t.Fatalf("pasteCount = %d, want 2", m.pasteCount)
	}

	// Enter and exit config mode
	m = typeString(m, "/config")
	m = sendKey(m, tea.KeyEnter)
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = result.(model)

	// Paste again — counter should continue from 2
	m = paste(m, longText)
	m = sendKey(m, tea.KeyEnter)

	if m.pasteCount != 3 {
		t.Errorf("pasteCount = %d, want 3 (should persist across config visits)", m.pasteCount)
	}

	// Verify correct paste ID in store
	expected := fmt.Sprintf("[pasted #3 | %d chars]", len(longText))
	_ = expected // used by paste detection logic
	if _, ok := m.pasteStore[3]; !ok {
		t.Error("pasteStore should have entry for paste #3")
	}
}
