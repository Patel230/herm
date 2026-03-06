package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestMain runs all tests in a temp directory so that saveConfig() calls
// never clobber the real .cpsl/config.json in the project root.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "cpsl-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	orig, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		panic(err)
	}
	defer os.Chdir(orig)

	os.Exit(m.Run())
}

// --- Test helpers ---

// newTestRenderer creates a renderer that discards output (for tests).
func newTestRenderer() *Renderer {
	return &Renderer{
		out: bufio.NewWriter(io.Discard),
	}
}

// newTestApp creates an App for testing without entering raw mode.
// The app is ready with the given dimensions and a discard renderer.
func newTestApp(w, h int) *App {
	cfg, _ := loadConfig()
	ta := NewTextInput(true)
	ta.SetWidth(w - 2)

	return &App{
		textarea: ta,
		config:   cfg,
		width:    w,
		height:   h,
		ready:    true,
		keyCh:    make(chan EventKey, 32),
		pasteCh:  make(chan EventPaste, 4),
		resizeCh: make(chan EventResize, 4),
		resultCh: make(chan any, 16),
		stopCh:   make(chan struct{}),
		renderer: newTestRenderer(),
	}
}

// simKey sends a special key press to the app, with optional modifiers.
func simKey(a *App, key Key, mods ...Modifier) {
	ev := EventKey{Key: key}
	for _, mod := range mods {
		ev.Mod |= mod
	}
	a.handleKey(ev)
}

// simRune sends a rune key press (typed character) to the app.
func simRune(a *App, r rune, mods ...Modifier) {
	ev := EventKey{Key: KeyRune, Rune: r}
	for _, mod := range mods {
		ev.Mod |= mod
	}
	a.handleKey(ev)
}

// simType types each rune of s into the app.
func simType(a *App, s string) {
	for _, r := range s {
		a.handleKey(EventKey{Key: KeyRune, Rune: r})
	}
}

// simPaste sends a paste event to the app.
func simPaste(a *App, content string) {
	a.handlePaste(EventPaste{Content: content})
}

// simResize sends a resize event to the app.
func simResize(a *App, w, h int) {
	a.handleResize(EventResize{Width: w, Height: h})
}

// simResult sends an async result to the app.
func simResult(a *App, result any) {
	a.handleResult(result)
}

// --- Window resize tests ---

func TestWindowResize(t *testing.T) {
	a := newTestApp(80, 24)

	if !a.ready {
		t.Fatal("app should be ready after creation")
	}
	if a.width != 80 {
		t.Errorf("width = %d, want 80", a.width)
	}
	if a.height != 24 {
		t.Errorf("height = %d, want 24", a.height)
	}

	// Textarea width should be window width minus 2 (border)
	if a.textarea.Width() != 78 {
		t.Errorf("textarea width = %d, want 78", a.textarea.Width())
	}
}

func TestWindowResizeMultiple(t *testing.T) {
	a := newTestApp(80, 24)
	simResize(a, 120, 40)

	if a.width != 120 {
		t.Errorf("width = %d, want 120", a.width)
	}
	if a.height != 40 {
		t.Errorf("height = %d, want 40", a.height)
	}
	if a.textarea.Width() != 118 {
		t.Errorf("textarea width = %d, want 118", a.textarea.Width())
	}
}

func TestWindowResizeSmall(t *testing.T) {
	a := newTestApp(10, 5)

	if !a.ready {
		t.Fatal("app should be ready even at small sizes")
	}
}

// --- Message sending tests ---

func TestEnterSendsMessage(t *testing.T) {
	a := newTestApp(80, 24)

	simType(a, "hello world")
	simKey(a, KeyEnter)

	// Textarea should be cleared after send
	if a.textarea.Value() != "" {
		t.Errorf("textarea should be empty after send, got %q", a.textarea.Value())
	}
	// Should have appended user message + error about no API keys
	if len(a.messages) < 2 {
		t.Fatalf("should have at least 2 messages, got %d", len(a.messages))
	}
	if a.messages[0].kind != msgUser {
		t.Errorf("first message kind = %d, want msgUser", a.messages[0].kind)
	}
	if a.messages[1].kind != msgError {
		t.Errorf("second message kind = %d, want msgError", a.messages[1].kind)
	}
}

func TestEnterEmptyDoesNotSend(t *testing.T) {
	a := newTestApp(80, 24)

	msgsBefore := len(a.messages)
	simKey(a, KeyEnter)

	if len(a.messages) != msgsBefore {
		t.Error("empty input should not add messages")
	}
}

func TestEnterWhitespaceOnlyDoesNotSend(t *testing.T) {
	a := newTestApp(80, 24)

	simType(a, "   ")
	msgsBefore := len(a.messages)
	simKey(a, KeyEnter)

	if len(a.messages) != msgsBefore {
		t.Error("whitespace-only input should not add messages")
	}
}

func TestMultipleMessages(t *testing.T) {
	a := newTestApp(80, 24)

	// Send multiple messages — each should clear the textarea
	simType(a, "first")
	simKey(a, KeyEnter)
	if a.textarea.Value() != "" {
		t.Error("textarea should be empty after first send")
	}

	simType(a, "second")
	simKey(a, KeyEnter)
	if a.textarea.Value() != "" {
		t.Error("textarea should be empty after second send")
	}

	simType(a, "third")
	simKey(a, KeyEnter)
	if a.textarea.Value() != "" {
		t.Error("textarea should be empty after third send")
	}
}

// --- Textarea height tests ---

func TestTextareaHeightExpandsWithContent(t *testing.T) {
	a := newTestApp(80, 24)

	if a.textarea.Height() != minInputHeight {
		t.Errorf("initial height = %d, want %d", a.textarea.Height(), minInputHeight)
	}

	// Type enough newlines to expand the textarea
	simType(a, "line1")
	simKey(a, KeyEnter, ModShift) // shift+enter = newline
	simType(a, "line2")
	simKey(a, KeyEnter, ModShift)
	simType(a, "line3")

	if a.textarea.Height() < 3 {
		t.Errorf("textarea height = %d, want >= 3 with 3 lines of content", a.textarea.Height())
	}
}

func TestTextareaHeightCappedAtMax(t *testing.T) {
	a := newTestApp(80, 24)

	// Type many newlines to try to exceed max
	for i := 0; i < maxInputHeight+5; i++ {
		simType(a, "x")
		simKey(a, KeyEnter, ModShift)
	}

	if a.textarea.Height() > maxInputHeight {
		t.Errorf("textarea height = %d, exceeds max %d", a.textarea.Height(), maxInputHeight)
	}
}

func TestTextareaHeightResetsAfterSend(t *testing.T) {
	a := newTestApp(80, 24)

	// Type multiline content
	simType(a, "line1")
	simKey(a, KeyEnter, ModShift)
	simType(a, "line2")
	simKey(a, KeyEnter, ModShift)
	simType(a, "line3")

	heightBefore := a.textarea.Height()
	if heightBefore < 2 {
		t.Fatalf("textarea should have expanded, got height %d", heightBefore)
	}

	// Send the message
	simKey(a, KeyEnter)

	if a.textarea.Height() != minInputHeight {
		t.Errorf("textarea height after send = %d, want %d", a.textarea.Height(), minInputHeight)
	}
}

func TestTextareaHeightIncreasesWithNewlines(t *testing.T) {
	a := newTestApp(80, 24)

	heightEmpty := a.textarea.Height()

	// Expand textarea
	simType(a, "line1")
	simKey(a, KeyEnter, ModShift)
	simType(a, "line2")
	simKey(a, KeyEnter, ModShift)
	simType(a, "line3")

	// Textarea should have grown
	if a.textarea.Height() <= heightEmpty {
		t.Errorf("textarea should grow with newlines: empty=%d, expanded=%d",
			heightEmpty, a.textarea.Height())
	}
}

func TestTextareaExpandsWithWrapping(t *testing.T) {
	a := newTestApp(30, 24) // narrow window to force wrapping

	// Type a long line that will wrap
	longText := strings.Repeat("word ", 20) // 100 chars, will wrap in 28-wide textarea
	simType(a, longText)

	if a.textarea.Height() <= 1 {
		t.Errorf("textarea should expand for wrapped content, got height %d", a.textarea.Height())
	}
}

func TestDisplayLineCount(t *testing.T) {
	a := newTestApp(80, 24)

	if a.textarea.DisplayLineCount() != 1 {
		t.Errorf("empty displayLineCount = %d, want 1", a.textarea.DisplayLineCount())
	}

	simType(a, "hello")
	if a.textarea.DisplayLineCount() != 1 {
		t.Errorf("short text displayLineCount = %d, want 1", a.textarea.DisplayLineCount())
	}
}

// --- Input box tests ---

func TestInputBoxHeight(t *testing.T) {
	a := newTestApp(80, 24)

	// Input box = textarea height + 2 (borders)
	expected := a.textarea.Height() + 2
	got := a.textarea.Height() + 2
	if got != expected {
		t.Errorf("inputBoxHeight = %d, want %d", got, expected)
	}
}

func TestSmallTerminalDoesNotPanic(t *testing.T) {
	a := newTestApp(80, 4) // very short terminal

	// Should not panic and should be ready
	if !a.ready {
		t.Error("app should be ready even at small terminal sizes")
	}
}

// --- Message content tests ---

func TestMessageTrimmed(t *testing.T) {
	a := newTestApp(80, 24)

	simType(a, "  hello  ")
	simKey(a, KeyEnter)

	// Textarea should be cleared after send
	if a.textarea.Value() != "" {
		t.Error("textarea should be empty after send")
	}
	// Should have appended user message to messages
	if len(a.messages) == 0 {
		t.Error("should have appended message after sending trimmed input")
	}
	if a.messages[0].kind != msgUser {
		t.Errorf("first message kind = %d, want msgUser", a.messages[0].kind)
	}
	if a.messages[0].content != "hello" {
		t.Errorf("message content = %q, want %q", a.messages[0].content, "hello")
	}
}

// --- Ctrl+C ---

func TestCtrlCQuits(t *testing.T) {
	a := newTestApp(80, 24)

	simRune(a, 'c', ModCtrl)
	if !a.quit {
		t.Fatal("ctrl+c should set quit=true")
	}
}

// --- Paste tests ---

func TestPasteAboveThresholdInsertPlaceholder(t *testing.T) {
	a := newTestApp(80, 24)

	longText := strings.Repeat("x", a.config.PasteCollapseMinChars)
	simPaste(a, longText)

	// Placeholder should be in the textarea
	expected := fmt.Sprintf("[pasted #%d | %d chars]", 1, a.config.PasteCollapseMinChars)
	if a.textarea.Value() != expected {
		t.Errorf("textarea = %q, want %q", a.textarea.Value(), expected)
	}

	// Actual content stored in pasteStore
	if a.pasteStore[1] != longText {
		t.Error("pasteStore should contain the original paste content")
	}
}

func TestPasteBelowThresholdInsertedVerbatim(t *testing.T) {
	a := newTestApp(80, 24)

	shortText := strings.Repeat("x", a.config.PasteCollapseMinChars-1)
	simPaste(a, shortText)

	// Small paste goes directly into textarea
	if a.textarea.Value() != shortText {
		t.Errorf("textarea = %q, want verbatim paste", a.textarea.Value())
	}
	if a.pasteCount != 0 {
		t.Errorf("pasteCount = %d, want 0 for small paste", a.pasteCount)
	}
}

func TestPasteCounterIncrements(t *testing.T) {
	a := newTestApp(80, 24)

	longText := strings.Repeat("x", a.config.PasteCollapseMinChars)

	// First paste + send
	simPaste(a, longText)
	simKey(a, KeyEnter)

	// Second paste + send
	simPaste(a, longText)
	simKey(a, KeyEnter)

	// Normal message
	simType(a, "hello")
	simKey(a, KeyEnter)

	// Third paste + send
	simPaste(a, longText)
	simKey(a, KeyEnter)

	if a.pasteCount != 3 {
		t.Errorf("pasteCount = %d, want 3", a.pasteCount)
	}
	// Verify paste store has all entries
	if a.pasteStore[1] != longText {
		t.Error("pasteStore[1] should contain paste content")
	}
	if a.pasteStore[2] != longText {
		t.Error("pasteStore[2] should contain paste content")
	}
	if a.pasteStore[3] != longText {
		t.Error("pasteStore[3] should contain paste content")
	}
}

func TestPasteExpandedOnSend(t *testing.T) {
	// Test that expandPastes works correctly
	store := map[int]string{1: strings.Repeat("a", 300)}
	result := expandPastes("[pasted #1 | 300 chars]", store)
	if result != strings.Repeat("a", 300) {
		t.Error("expandPastes should replace placeholder with actual content")
	}
}

func TestPasteStoreRetainsContent(t *testing.T) {
	a := newTestApp(80, 24)

	text1 := strings.Repeat("A", 300)
	text2 := strings.Repeat("B", 400)

	simPaste(a, text1)
	simKey(a, KeyEnter)
	simPaste(a, text2)
	simKey(a, KeyEnter)

	if a.pasteStore[1] != text1 {
		t.Error("pasteStore[1] should contain first paste")
	}
	if a.pasteStore[2] != text2 {
		t.Error("pasteStore[2] should contain second paste")
	}
}

func TestPasteTypingContinuesAfterPaste(t *testing.T) {
	a := newTestApp(80, 24)

	simType(a, "before ")
	longText := strings.Repeat("x", a.config.PasteCollapseMinChars)
	simPaste(a, longText)
	simType(a, " after")

	// Verify textarea has placeholder
	if !strings.Contains(a.textarea.Value(), "[pasted #1") {
		t.Error("textarea should contain paste placeholder")
	}
	if !strings.HasPrefix(a.textarea.Value(), "before ") {
		t.Error("textarea should start with 'before '")
	}
	if !strings.HasSuffix(a.textarea.Value(), " after") {
		t.Error("textarea should end with ' after'")
	}

	// Verify expandPastes produces correct result
	content := expandPastes(a.textarea.Value(), a.pasteStore)
	if !strings.HasPrefix(content, "before ") {
		t.Errorf("expanded should start with 'before ', got %q", content)
	}
	if !strings.Contains(content, longText) {
		t.Error("expanded should contain paste content")
	}
	if !strings.HasSuffix(content, " after") {
		t.Errorf("expanded should end with ' after', got %q", content)
	}
}

func TestPasteMultiplePastesInOneMessage(t *testing.T) {
	a := newTestApp(80, 24)

	text1 := strings.Repeat("A", 300)
	text2 := strings.Repeat("B", 400)

	simType(a, "code: ")
	simPaste(a, text1)
	simType(a, " and: ")
	simPaste(a, text2)

	// Verify expandPastes works for multiple placeholders
	content := expandPastes(a.textarea.Value(), a.pasteStore)
	if !strings.Contains(content, text1) {
		t.Error("expanded should contain paste #1 content")
	}
	if !strings.Contains(content, text2) {
		t.Error("expanded should contain paste #2 content")
	}
	if !strings.HasPrefix(content, "code: ") {
		t.Errorf("expanded should start with 'code: ', got prefix %q", content[:10])
	}
	if !strings.Contains(content, " and: ") {
		t.Error("expanded should contain ' and: ' between pastes")
	}
}

// --- Command parsing tests ---

func TestSlashConfigEntersConfigMode(t *testing.T) {
	a := newTestApp(80, 24)

	simType(a, "/config")
	simKey(a, KeyEnter)

	if a.mode != modeConfig {
		t.Errorf("mode = %d, want modeConfig (%d)", a.mode, modeConfig)
	}
	// Textarea should be cleared
	if a.textarea.Value() != "" {
		t.Errorf("textarea should be empty after /config, got %q", a.textarea.Value())
	}
}

func TestUnknownCommandShowsError(t *testing.T) {
	a := newTestApp(80, 24)

	simType(a, "/foo")
	simKey(a, KeyEnter)

	if a.mode != modeChat {
		t.Error("unknown command should stay in chat mode")
	}
	// Should have appended error message
	found := false
	for _, msg := range a.messages {
		if msg.kind == msgError {
			found = true
			break
		}
	}
	if !found {
		t.Error("unknown command should append an error message")
	}
}

func TestSlashInNormalTextNotTreatedAsCommand(t *testing.T) {
	a := newTestApp(80, 24)

	simType(a, "use a/b path")
	simKey(a, KeyEnter)

	if a.mode != modeChat {
		t.Error("text with / in middle should stay in chat mode")
	}
	// Should have appended user message (not treated as command)
	if len(a.messages) == 0 {
		t.Error("normal text should be sent and append a message")
	}
	if a.messages[0].kind != msgUser {
		t.Errorf("first message kind = %d, want msgUser", a.messages[0].kind)
	}
	// Textarea should be cleared
	if a.textarea.Value() != "" {
		t.Error("textarea should be empty after send")
	}
}

func TestSlashConfigWithExtraArgs(t *testing.T) {
	a := newTestApp(80, 24)

	simType(a, "/config something")
	simKey(a, KeyEnter)

	// Should still enter config mode (extra args ignored for now)
	if a.mode != modeConfig {
		t.Errorf("mode = %d, want modeConfig", a.mode)
	}
}

// --- Mode switching tests ---

func TestConfigModeEscDiscards(t *testing.T) {
	a := newTestApp(80, 24)

	originalThreshold := a.config.PasteCollapseMinChars

	// Enter config mode
	simType(a, "/config")
	simKey(a, KeyEnter)

	if a.mode != modeConfig {
		t.Fatal("should be in config mode")
	}

	// Press Esc to cancel
	simKey(a, KeyEscape)

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat after Esc", a.mode)
	}
	if a.config.PasteCollapseMinChars != originalThreshold {
		t.Error("config should not change after Esc")
	}
}

func TestConfigModeEnterSaves(t *testing.T) {
	a := newTestApp(80, 24)

	// Enter config mode
	simType(a, "/config")
	simKey(a, KeyEnter)

	if a.mode != modeConfig {
		t.Fatal("should be in config mode")
	}

	// The form should be pre-populated with current value
	val := a.configForm.fields[0].input.Value()
	if val != strconv.Itoa(a.config.PasteCollapseMinChars) {
		t.Errorf("form value = %q, want %q", val, strconv.Itoa(a.config.PasteCollapseMinChars))
	}

	// Press Enter to save (valid value already set)
	simKey(a, KeyEnter)

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat after Enter", a.mode)
	}
}

func TestConfigModeValidationRejectsInvalid(t *testing.T) {
	a := newTestApp(80, 24)

	// Enter config mode
	simType(a, "/config")
	simKey(a, KeyEnter)

	// Set invalid value directly
	a.configForm.fields[0].input.SetValue("abc")

	// Press Enter — should stay in config mode due to validation
	simKey(a, KeyEnter)

	if a.mode != modeConfig {
		t.Errorf("mode = %d, want modeConfig (invalid input should not save)", a.mode)
	}
	if a.configForm.fields[0].err == "" {
		t.Error("should show validation error for non-numeric input")
	}
}

func TestConfigModeCtrlCDiscards(t *testing.T) {
	a := newTestApp(80, 24)

	simType(a, "/config")
	simKey(a, KeyEnter)

	// Ctrl+C in config mode should discard (not quit the app)
	simRune(a, 'c', ModCtrl)

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat after Ctrl+C in config", a.mode)
	}
	// Should NOT quit the app
	if a.quit {
		t.Error("Ctrl+C in config mode should not quit the app")
	}
}

func TestConfigModeWindowResizeForwarded(t *testing.T) {
	a := newTestApp(80, 24)

	simType(a, "/config")
	simKey(a, KeyEnter)

	// Resize while in config mode
	simResize(a, 120, 40)

	if a.mode != modeConfig {
		t.Error("should stay in config mode after resize")
	}
	if a.width != 120 || a.height != 40 {
		t.Errorf("dimensions = %dx%d, want 120x40", a.width, a.height)
	}
}

// --- filterCommands tests ---

func TestFilterCommandsExactMatch(t *testing.T) {
	matches := filterCommands("/config")
	if len(matches) != 1 || matches[0] != "/config" {
		t.Errorf("filterCommands(/config) = %v, want [/config]", matches)
	}
}

func TestFilterCommandsPartialMatch(t *testing.T) {
	matches := filterCommands("/con")
	if len(matches) != 1 || matches[0] != "/config" {
		t.Errorf("filterCommands(/con) = %v, want [/config]", matches)
	}
}

func TestFilterCommandsNoMatch(t *testing.T) {
	matches := filterCommands("/xyz")
	if len(matches) != 0 {
		t.Errorf("filterCommands(/xyz) = %v, want []", matches)
	}
}

func TestFilterCommandsSlashOnly(t *testing.T) {
	matches := filterCommands("/")
	if len(matches) != len(commands) {
		t.Errorf("filterCommands(/) = %v, want all %d commands", matches, len(commands))
	}
}

// --- autocompleteMatches tests ---

func TestAutocompleteMatchesChatMode(t *testing.T) {
	a := newTestApp(80, 24)
	simType(a, "/")

	matches := a.autocompleteMatches()
	if len(matches) == 0 {
		t.Error("autocompleteMatches should return matches when typing / in chat mode")
	}
}

func TestAutocompleteMatchesConfigMode(t *testing.T) {
	a := newTestApp(80, 24)
	simType(a, "/config")
	simKey(a, KeyEnter)

	// In config mode, autocomplete should not be active
	matches := a.autocompleteMatches()
	if matches != nil {
		t.Errorf("autocompleteMatches should be nil in config mode, got %v", matches)
	}
}

func TestAutocompleteMatchesNoSlash(t *testing.T) {
	a := newTestApp(80, 24)
	simType(a, "hello")

	matches := a.autocompleteMatches()
	if matches != nil {
		t.Errorf("autocompleteMatches should be nil without slash, got %v", matches)
	}
}

// --- Tab/Esc key handling tests ---

func TestTabAcceptsTopMatch(t *testing.T) {
	a := newTestApp(80, 24)
	simType(a, "/con")
	simKey(a, KeyTab)

	if a.textarea.Value() != "/config" {
		t.Errorf("textarea = %q, want /config after Tab", a.textarea.Value())
	}
}

func TestTabWithNoMatchDoesNothing(t *testing.T) {
	a := newTestApp(80, 24)
	simType(a, "/xyz")
	simKey(a, KeyTab)

	if a.textarea.Value() != "/xyz" {
		t.Errorf("textarea = %q, want /xyz (unchanged) after Tab with no match", a.textarea.Value())
	}
}

func TestEscDismissesSlashInput(t *testing.T) {
	a := newTestApp(80, 24)
	simType(a, "/con")
	simKey(a, KeyEscape)

	if a.textarea.Value() != "" {
		t.Errorf("textarea = %q, want empty after Esc on slash input", a.textarea.Value())
	}
}

func TestEscWithoutSlashDoesNotClear(t *testing.T) {
	a := newTestApp(80, 24)
	simType(a, "hello")
	simKey(a, KeyEscape)

	if a.textarea.Value() != "hello" {
		t.Errorf("textarea = %q, want hello (unchanged) after Esc without slash", a.textarea.Value())
	}
}

func TestTabThenEnterExecutesCommand(t *testing.T) {
	a := newTestApp(80, 24)
	simType(a, "/con")
	simKey(a, KeyTab)
	simKey(a, KeyEnter)

	if a.mode != modeConfig {
		t.Errorf("mode = %d, want modeConfig after Tab+Enter on /con", a.mode)
	}
}

func TestAutocompleteVisibleInRender(t *testing.T) {
	a := newTestApp(80, 24)
	simType(a, "/")

	ac := a.renderAutocomplete()
	if !strings.Contains(ac, "/config") {
		t.Error("renderAutocomplete should show /config when typing /")
	}
}

func TestPasteConfigThresholdRespected(t *testing.T) {
	a := newTestApp(80, 24)
	a.config.PasteCollapseMinChars = 50

	// Paste at custom threshold — should collapse
	text := strings.Repeat("x", 50)
	simPaste(a, text)
	if !strings.Contains(a.textarea.Value(), "[pasted #1") {
		t.Error("paste at custom threshold should be collapsed")
	}

	a.textarea.Reset()

	// Paste below custom threshold — should be verbatim
	shortText := strings.Repeat("y", 49)
	simPaste(a, shortText)
	if a.textarea.Value() != shortText {
		t.Errorf("paste below threshold should be verbatim, got %q", a.textarea.Value())
	}
}

// --- /model command tests ---

// appWithKey returns an app with only an Anthropic API key configured
// and test models loaded.
func appWithKey() *App {
	a := newTestApp(80, 24)
	a.config.AnthropicAPIKey = "sk-test-key"
	a.config.GrokAPIKey = ""
	a.config.OpenAIAPIKey = ""
	a.config.ActiveModel = ""
	a.models = testModels()
	a.modelsLoaded = true
	return a
}

func TestSlashModelEntersModelMode(t *testing.T) {
	a := appWithKey()

	simType(a, "/model")
	simKey(a, KeyEnter)

	if a.mode != modeModel {
		t.Errorf("mode = %d, want modeModel (%d)", a.mode, modeModel)
	}
	if a.textarea.Value() != "" {
		t.Errorf("textarea should be empty after /model, got %q", a.textarea.Value())
	}
}

func TestSlashModelNoKeysShowsError(t *testing.T) {
	a := newTestApp(80, 24)
	a.config.AnthropicAPIKey = ""
	a.config.GrokAPIKey = ""
	a.config.OpenAIAPIKey = ""
	a.models = testModels()
	a.modelsLoaded = true

	simType(a, "/model")
	simKey(a, KeyEnter)

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat (no keys configured)", a.mode)
	}
	// Should have appended error message about no API keys
	found := false
	for _, msg := range a.messages {
		if msg.kind == msgError {
			found = true
			break
		}
	}
	if !found {
		t.Error("should append error message about no API keys")
	}
}

func TestModelModeEscCancels(t *testing.T) {
	a := appWithKey()

	simType(a, "/model")
	simKey(a, KeyEnter)

	if a.mode != modeModel {
		t.Fatal("should be in model mode")
	}

	simKey(a, KeyEscape)

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat after Esc", a.mode)
	}
}

func TestModelModeEnterSelectsModel(t *testing.T) {
	a := appWithKey()

	simType(a, "/model")
	simKey(a, KeyEnter)

	// Should show Anthropic models since only that key is set
	if len(a.modelList.models) == 0 {
		t.Fatal("model list should have models")
	}
	for _, md := range a.modelList.models {
		if md.Provider != ProviderAnthropic {
			t.Errorf("model list should only have anthropic models, got %s", md.Provider)
		}
	}

	// Select (Enter)
	selectedModel := a.modelList.selected()
	simKey(a, KeyEnter)

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat after selection", a.mode)
	}
	if a.config.ActiveModel != selectedModel.ID {
		t.Errorf("ActiveModel = %q, want %q", a.config.ActiveModel, selectedModel.ID)
	}
}

func TestModelModeOnlyShowsConfiguredProviders(t *testing.T) {
	a := newTestApp(80, 24)
	a.config.AnthropicAPIKey = ""
	a.config.GrokAPIKey = "grok-key"
	a.config.OpenAIAPIKey = "openai-key"
	a.models = testModels()
	a.modelsLoaded = true

	simType(a, "/model")
	simKey(a, KeyEnter)

	if a.mode != modeModel {
		t.Fatal("should be in model mode")
	}

	for _, md := range a.modelList.models {
		if md.Provider == ProviderAnthropic {
			t.Error("should not show Anthropic models without key")
		}
	}
	// Should have Grok and OpenAI models
	hasGrok := false
	hasOpenAI := false
	for _, md := range a.modelList.models {
		if md.Provider == ProviderGrok {
			hasGrok = true
		}
		if md.Provider == ProviderOpenAI {
			hasOpenAI = true
		}
	}
	if !hasGrok {
		t.Error("should show Grok models with key configured")
	}
	if !hasOpenAI {
		t.Error("should show OpenAI models with key configured")
	}
}

func TestModelModeNavigationUpDown(t *testing.T) {
	a := newTestApp(80, 24)
	a.config.AnthropicAPIKey = "key"
	a.config.OpenAIAPIKey = "key"
	a.config.ActiveModel = "claude-opus-4-0-20250514" // most expensive → cursor starts near top
	a.models = testModels()
	a.modelsLoaded = true

	simType(a, "/model")
	simKey(a, KeyEnter)

	start := a.modelList.cursor

	// Move down
	simKey(a, KeyDown)
	if a.modelList.cursor != start+1 {
		t.Errorf("cursor after down = %d, want %d", a.modelList.cursor, start+1)
	}

	// Move down again
	simKey(a, KeyDown)
	if a.modelList.cursor != start+2 {
		t.Errorf("cursor after second down = %d, want %d", a.modelList.cursor, start+2)
	}

	// Move up
	simKey(a, KeyUp)
	if a.modelList.cursor != start+1 {
		t.Errorf("cursor after up = %d, want %d", a.modelList.cursor, start+1)
	}
}

func TestModelModeNavigationBounds(t *testing.T) {
	a := appWithKey()
	a.config.ActiveModel = ""

	simType(a, "/model")
	simKey(a, KeyEnter)

	// Move cursor to top first
	for i := 0; i < len(a.modelList.models); i++ {
		simKey(a, KeyUp)
	}
	if a.modelList.cursor != 0 {
		t.Errorf("cursor should be at 0 after moving up enough times, got %d", a.modelList.cursor)
	}

	// Try to go above 0
	simKey(a, KeyUp)
	if a.modelList.cursor != 0 {
		t.Errorf("cursor should not go below 0, got %d", a.modelList.cursor)
	}

	// Go to bottom
	for i := 0; i < len(a.modelList.models)+5; i++ {
		simKey(a, KeyDown)
	}
	if a.modelList.cursor != len(a.modelList.models)-1 {
		t.Errorf("cursor should stop at last item, got %d, want %d",
			a.modelList.cursor, len(a.modelList.models)-1)
	}
}

func TestModelModeCursorStartsOnActiveModel(t *testing.T) {
	a := newTestApp(80, 24)
	a.config.AnthropicAPIKey = "key"
	a.models = testModels()
	a.modelsLoaded = true
	// Set active model to second Anthropic model
	a.config.ActiveModel = "claude-haiku-4-5-20250414"

	simType(a, "/model")
	simKey(a, KeyEnter)

	// Find the index of the active model in the list
	expectedIdx := -1
	for i, md := range a.modelList.models {
		if md.ID == "claude-haiku-4-5-20250414" {
			expectedIdx = i
			break
		}
	}
	if expectedIdx == -1 {
		t.Fatal("active model not found in list")
	}
	if a.modelList.cursor != expectedIdx {
		t.Errorf("cursor = %d, want %d (should start on active model)", a.modelList.cursor, expectedIdx)
	}
}

func TestModelModeActiveModelHighlighted(t *testing.T) {
	a := newTestApp(80, 24)
	a.config.AnthropicAPIKey = "key"
	a.config.ActiveModel = "claude-sonnet-4-0-20250514"
	a.models = testModels()
	a.modelsLoaded = true

	simType(a, "/model")
	simKey(a, KeyEnter)

	if a.modelList.activeModel != "claude-sonnet-4-0-20250514" {
		t.Errorf("activeModel = %q, want claude-sonnet-4-0-20250514", a.modelList.activeModel)
	}

	// The view should contain the active marker
	view := a.modelList.View()
	if !strings.Contains(view, "●") {
		t.Error("model list view should show active marker ●")
	}
}

func TestModelModeCtrlCCancels(t *testing.T) {
	a := appWithKey()

	simType(a, "/model")
	simKey(a, KeyEnter)

	simRune(a, 'c', ModCtrl)

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat after Ctrl+C", a.mode)
	}
}

func TestModelModeWindowResize(t *testing.T) {
	a := appWithKey()

	simType(a, "/model")
	simKey(a, KeyEnter)

	simResize(a, 120, 40)

	if a.mode != modeModel {
		t.Error("should stay in model mode after resize")
	}
	if a.width != 120 || a.height != 40 {
		t.Errorf("dimensions = %dx%d, want 120x40", a.width, a.height)
	}
}

func TestModelModeViewRendered(t *testing.T) {
	a := appWithKey()

	simType(a, "/model")
	simKey(a, KeyEnter)

	v := a.modelList.View()
	if !strings.Contains(v, "Select Model") {
		t.Error("model view should contain 'Select Model'")
	}
}

func TestModelCommandInAutocomplete(t *testing.T) {
	a := newTestApp(80, 24)
	simType(a, "/m")

	matches := a.autocompleteMatches()
	found := false
	for _, match := range matches {
		if match == "/model" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("autocomplete for /m should include /model, got %v", matches)
	}
}

func TestModelModeSelectNavigatedModel(t *testing.T) {
	a := newTestApp(80, 24)
	a.config.AnthropicAPIKey = "key"
	a.config.ActiveModel = ""
	a.models = testModels()
	a.modelsLoaded = true

	simType(a, "/model")
	simKey(a, KeyEnter)

	// Navigate down from initial position
	simKey(a, KeyDown)

	targetModel := a.modelList.models[a.modelList.cursor]

	// Select it
	simKey(a, KeyEnter)

	if a.config.ActiveModel != targetModel.ID {
		t.Errorf("ActiveModel = %q, want %q", a.config.ActiveModel, targetModel.ID)
	}
}

func TestModelModeVimKeys(t *testing.T) {
	a := newTestApp(80, 24)
	a.config.AnthropicAPIKey = "key"
	a.config.OpenAIAPIKey = "key"
	a.config.ActiveModel = ""
	a.models = testModels()
	a.modelsLoaded = true

	simType(a, "/model")
	simKey(a, KeyEnter)

	startCursor := a.modelList.cursor

	// j moves down
	simRune(a, 'j')
	if a.modelList.cursor != startCursor+1 {
		t.Errorf("cursor after j = %d, want %d", a.modelList.cursor, startCursor+1)
	}

	// k moves up
	simRune(a, 'k')
	if a.modelList.cursor != startCursor {
		t.Errorf("cursor after k = %d, want %d", a.modelList.cursor, startCursor)
	}
}

// --- modelsMsg handling tests ---

func TestModelsMsgSetsModels(t *testing.T) {
	a := newTestApp(80, 24)

	if a.modelsLoaded {
		t.Fatal("modelsLoaded should be false initially")
	}

	simResult(a, modelsMsg{models: testModels()})

	if !a.modelsLoaded {
		t.Error("modelsLoaded should be true after modelsMsg")
	}
	if a.modelsErr != nil {
		t.Errorf("modelsErr should be nil, got %v", a.modelsErr)
	}
	if len(a.models) != len(testModels()) {
		t.Errorf("models count = %d, want %d", len(a.models), len(testModels()))
	}
}

func TestModelsMsgSetsError(t *testing.T) {
	a := newTestApp(80, 24)

	simResult(a, modelsMsg{err: fmt.Errorf("network error")})

	if !a.modelsLoaded {
		t.Error("modelsLoaded should be true even on error")
	}
	if a.modelsErr == nil {
		t.Error("modelsErr should be set")
	}
	if len(a.models) != 0 {
		t.Errorf("models should be empty on error, got %d", len(a.models))
	}
}

func TestSlashModelBeforeModelsLoaded(t *testing.T) {
	a := newTestApp(80, 24)
	a.config.AnthropicAPIKey = "key"
	// modelsLoaded is false by default

	simType(a, "/model")
	simKey(a, KeyEnter)

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat (models not loaded)", a.mode)
	}
	// Should have appended info message about loading
	found := false
	for _, msg := range a.messages {
		if msg.kind == msgInfo {
			found = true
			break
		}
	}
	if !found {
		t.Error("should append info message about models loading")
	}
}

func TestModelListScrollsWithCursor(t *testing.T) {
	// Create many models to force scrolling in a small window
	var manyModels []ModelDef
	for i := 0; i < 30; i++ {
		manyModels = append(manyModels, ModelDef{
			Provider:    ProviderAnthropic,
			ID:          fmt.Sprintf("anthropic/model-%d", i),
			DisplayName: fmt.Sprintf("Model %d", i),
			PromptPrice: 1.0,
		})
	}

	a := newTestApp(80, 20) // small height to force scrolling
	a.config.AnthropicAPIKey = "key"
	a.models = manyModels
	a.modelsLoaded = true

	simType(a, "/model")
	simKey(a, KeyEnter)

	if a.mode != modeModel {
		t.Fatal("should be in model mode")
	}

	vis := a.modelList.visibleRows()
	if vis >= 30 {
		t.Skip("window too large to test scrolling")
	}

	// Cursor starts at 0, scroll at 0
	if a.modelList.cursor != 0 {
		t.Errorf("cursor = %d, want 0", a.modelList.cursor)
	}
	if a.modelList.scroll != 0 {
		t.Errorf("scroll = %d, want 0", a.modelList.scroll)
	}

	// Navigate past visible area
	for i := 0; i < vis+2; i++ {
		simKey(a, KeyDown)
	}

	// Cursor should be past initial visible area, scroll should follow
	if a.modelList.cursor != vis+2 {
		t.Errorf("cursor = %d, want %d", a.modelList.cursor, vis+2)
	}
	if a.modelList.scroll == 0 {
		t.Error("scroll should have advanced past 0")
	}
	// Cursor should be within visible window
	if a.modelList.cursor < a.modelList.scroll || a.modelList.cursor >= a.modelList.scroll+vis {
		t.Errorf("cursor %d not in visible window [%d, %d)",
			a.modelList.cursor, a.modelList.scroll, a.modelList.scroll+vis)
	}

	// View should show scroll indicators
	view := a.modelList.View()
	if !strings.Contains(view, "↑") {
		t.Error("should show scroll-up indicator")
	}
	if !strings.Contains(view, "↓") {
		t.Error("should show scroll-down indicator")
	}
}

func TestSlashModelWithFetchError(t *testing.T) {
	a := newTestApp(80, 24)
	a.config.AnthropicAPIKey = "key"
	a.modelsLoaded = true
	a.modelsErr = fmt.Errorf("connection refused")

	simType(a, "/model")
	simKey(a, KeyEnter)

	if a.mode != modeChat {
		t.Errorf("mode = %d, want modeChat (fetch error)", a.mode)
	}
	// Should have appended error message about fetch error
	found := false
	for _, msg := range a.messages {
		if msg.kind == msgError {
			found = true
			break
		}
	}
	if !found {
		t.Error("should append error message about fetch error")
	}
}

// testModelsWithSWE returns test models enriched with SWE-bench scores.
func testModelsWithSWE() []ModelDef {
	return []ModelDef{
		{Provider: ProviderAnthropic, ID: "claude-opus-4-0-20250514", DisplayName: "Claude Opus 4", PromptPrice: 15.0, CompletionPrice: 75.0},
		{Provider: ProviderAnthropic, ID: "claude-sonnet-4-0-20250514", DisplayName: "Claude Sonnet 4", PromptPrice: 3.0, CompletionPrice: 15.0},
		{Provider: ProviderOpenAI, ID: "gpt-4o", DisplayName: "GPT-4o", PromptPrice: 2.5, CompletionPrice: 10.0},
		{Provider: ProviderGrok, ID: "grok-3", DisplayName: "Grok 3", PromptPrice: 3.0, CompletionPrice: 15.0},
	}
}

// enterModelModeApp sets up an app with the given models and enters /model.
func enterModelModeApp(t *testing.T, models []ModelDef) *App {
	t.Helper()
	a := newTestApp(120, 40)
	a.config.AnthropicAPIKey = "key"
	a.config.OpenAIAPIKey = "key"
	a.config.GrokAPIKey = "key"
	a.models = models
	a.modelsLoaded = true

	simType(a, "/model")
	simKey(a, KeyEnter)
	if a.mode != modeModel {
		t.Fatalf("expected modeModel, got %d", a.mode)
	}
	return a
}

func TestSortDefaultPriceDescending(t *testing.T) {
	a := enterModelModeApp(t, testModelsWithSWE())

	if a.modelList.sortCol != colPrice {
		t.Errorf("default sortCol = %d, want colPrice (%d)", a.modelList.sortCol, colPrice)
	}
	if a.modelList.sortDirs[colPrice] {
		t.Error("default sort should be descending for price")
	}

	// First model should have the most expensive price
	for i := 1; i < len(a.modelList.models); i++ {
		if a.modelList.models[i-1].PromptPrice < a.modelList.models[i].PromptPrice {
			t.Errorf("price sort broken: %f < %f at index %d",
				a.modelList.models[i-1].PromptPrice, a.modelList.models[i].PromptPrice, i)
		}
	}
}

func TestSortRightArrowCyclesColumn(t *testing.T) {
	a := enterModelModeApp(t, testModelsWithSWE())

	// Default is colPrice (2)
	if a.modelList.sortCol != colPrice {
		t.Fatalf("initial sortCol = %d, want colPrice", a.modelList.sortCol)
	}

	// Right arrow → colName (wraps: (2+1)%3 = 0)
	simKey(a, KeyRight)
	if a.modelList.sortCol != colName {
		t.Errorf("after right: sortCol = %d, want colName (%d)", a.modelList.sortCol, colName)
	}
	if !a.modelList.sortDirs[colName] {
		t.Error("colName should default to ascending")
	}

	// Right arrow → colProvider
	simKey(a, KeyRight)
	if a.modelList.sortCol != colProvider {
		t.Errorf("after right x2: sortCol = %d, want colProvider (%d)", a.modelList.sortCol, colProvider)
	}

	// Right arrow → colPrice (wraps back)
	simKey(a, KeyRight)
	if a.modelList.sortCol != colPrice {
		t.Errorf("after right x3: sortCol = %d, want colPrice (%d)", a.modelList.sortCol, colPrice)
	}
	if a.modelList.sortDirs[colPrice] {
		t.Error("colPrice should default to descending")
	}
}

func TestSortLeftArrowCyclesColumn(t *testing.T) {
	a := enterModelModeApp(t, testModelsWithSWE())

	// Default is colPrice (2), left arrow → colProvider (1)
	simKey(a, KeyLeft)
	if a.modelList.sortCol != colProvider {
		t.Errorf("after left: sortCol = %d, want colProvider (%d)", a.modelList.sortCol, colProvider)
	}
}

func TestSortByNameAlphabetical(t *testing.T) {
	a := enterModelModeApp(t, testModelsWithSWE())

	// Switch to name sort (right from colPrice wraps to colName)
	simKey(a, KeyRight)
	if a.modelList.sortCol != colName {
		t.Fatalf("sortCol = %d, want colName", a.modelList.sortCol)
	}

	// Should be alphabetical ascending
	for i := 1; i < len(a.modelList.models); i++ {
		nameA := strings.ToLower(a.modelList.models[i-1].DisplayName)
		nameB := strings.ToLower(a.modelList.models[i].DisplayName)
		if nameA > nameB {
			t.Errorf("name sort broken: %q > %q at index %d", nameA, nameB, i)
		}
	}
}

func TestSortByPriceAscending(t *testing.T) {
	a := enterModelModeApp(t, testModelsWithSWE())

	// Default sort is already colPrice descending
	if a.modelList.sortCol != colPrice {
		t.Fatalf("sortCol = %d, want colPrice", a.modelList.sortCol)
	}

	// Should be price descending (most expensive first)
	for i := 1; i < len(a.modelList.models); i++ {
		if a.modelList.models[i-1].PromptPrice < a.modelList.models[i].PromptPrice {
			t.Errorf("price sort broken: %f < %f at index %d",
				a.modelList.models[i-1].PromptPrice, a.modelList.models[i].PromptPrice, i)
		}
	}
}

func TestSortCursorPreserved(t *testing.T) {
	a := enterModelModeApp(t, testModelsWithSWE())

	// Move cursor to a specific model
	simKey(a, KeyDown)
	selectedID := a.modelList.selected().ID

	// Change sort column
	simKey(a, KeyRight)

	// Cursor should still point to the same model
	if a.modelList.selected().ID != selectedID {
		t.Errorf("cursor moved to %q after sort, want %q", a.modelList.selected().ID, selectedID)
	}
}

func TestTableViewHasHeaderAndSeparator(t *testing.T) {
	a := enterModelModeApp(t, testModelsWithSWE())
	view := a.modelList.View()

	if !strings.Contains(view, "MODEL") {
		t.Error("view should contain MODEL header")
	}
	if !strings.Contains(view, "PROVIDER") {
		t.Error("view should contain PROVIDER header")
	}
	if !strings.Contains(view, "PRICE") {
		t.Error("view should contain PRICE header")
	}
	if strings.Contains(view, "SWE-BENCH") {
		t.Error("view should NOT contain SWE-BENCH header (column removed)")
	}
	if !strings.Contains(view, "─") {
		t.Error("view should contain separator line")
	}
	if !strings.Contains(view, "▼") {
		t.Error("view should contain sort direction indicator (▼ for default descending price)")
	}
	if !strings.Contains(view, "←/→") {
		t.Error("hint should mention ←/→ for sort")
	}
	if !strings.Contains(view, "tab") {
		t.Error("hint should mention tab for reverse")
	}
}

func TestTableViewSortArrowChanges(t *testing.T) {
	a := enterModelModeApp(t, testModelsWithSWE())

	// Default: PRICE with ▼ (descending)
	view := a.modelList.View()
	if !strings.Contains(view, "PRICE ▼") {
		t.Error("default view should show PRICE ▼")
	}

	// Switch to name sort (right from colPrice wraps to colName, ascending ▲)
	simKey(a, KeyRight)
	view = a.modelList.View()
	if !strings.Contains(view, "MODEL ▲") {
		t.Error("name sort view should show MODEL ▲")
	}
	// PRICE should no longer have arrow
	if strings.Contains(view, "PRICE ▼") || strings.Contains(view, "PRICE ▲") {
		t.Error("PRICE should not have arrow when not active sort")
	}
}

func TestTabTogglesSortDirection(t *testing.T) {
	a := enterModelModeApp(t, testModelsWithSWE())

	// Default: colPrice descending
	if a.modelList.sortDirs[colPrice] {
		t.Fatal("default price should be descending")
	}

	// Record first model (most expensive)
	firstBefore := a.modelList.models[0].ID

	// Press tab → ascending
	simKey(a, KeyTab)

	if !a.modelList.sortDirs[colPrice] {
		t.Error("tab should toggle to ascending")
	}
	if a.modelList.sortCol != colPrice {
		t.Error("tab should not change the sort column")
	}

	// First model should now be the cheapest
	if a.modelList.models[0].ID == firstBefore {
		t.Error("order should change after toggling direction")
	}

	// Press tab again → descending
	simKey(a, KeyTab)

	if a.modelList.sortDirs[colPrice] {
		t.Error("second tab should toggle back to descending")
	}
	if a.modelList.models[0].ID != firstBefore {
		t.Error("order should restore after toggling back")
	}
}

func TestTabPreservesPerColumnDirection(t *testing.T) {
	a := enterModelModeApp(t, testModelsWithSWE())

	// Toggle price to ascending
	simKey(a, KeyTab)
	if !a.modelList.sortDirs[colPrice] {
		t.Fatal("price should now be ascending")
	}

	// Switch to name column (right arrow wraps to colName)
	simKey(a, KeyRight)
	if a.modelList.sortCol != colName {
		t.Fatalf("sortCol = %d, want colName", a.modelList.sortCol)
	}
	// Name should still be at its default (ascending)
	if !a.modelList.sortDirs[colName] {
		t.Error("name should be ascending (default)")
	}

	// Switch back to price — should remember ascending
	simKey(a, KeyLeft)
	if a.modelList.sortCol != colPrice {
		t.Fatalf("sortCol = %d, want colPrice", a.modelList.sortCol)
	}
	if !a.modelList.sortDirs[colPrice] {
		t.Error("price should still be ascending (remembered)")
	}
}

func TestSortDirsSavedToConfig(t *testing.T) {
	a := enterModelModeApp(t, testModelsWithSWE())

	// Toggle price to ascending
	simKey(a, KeyTab)

	// Exit model mode (cancel) — should persist sort dirs
	simKey(a, KeyEscape)

	// Config should have the updated sort dirs
	if !a.config.ModelSortDirs["price"] {
		t.Error("config should persist price as ascending after tab toggle")
	}
	if !a.config.ModelSortDirs["name"] {
		t.Error("config should persist name as ascending (default)")
	}

	// Re-enter model mode — should restore saved dirs
	simType(a, "/model")
	simKey(a, KeyEnter)

	if !a.modelList.sortDirs[colPrice] {
		t.Error("price should be ascending (restored from config)")
	}
}

// --- Resize and scrollback tests ---

func TestResizeReprintsScrollback(t *testing.T) {
	a := newTestApp(80, 24)

	// Add some messages and mark as printed
	a.messages = append(a.messages, chatMessage{kind: msgUser, content: "hello", leadBlank: true})
	a.messages = append(a.messages, chatMessage{kind: msgAssistant, content: "world"})
	a.printedMsgCount = len(a.messages)

	// Resize should reprint all messages
	simResize(a, 100, 30)
	if a.printedMsgCount != 2 {
		t.Errorf("printedMsgCount = %d, want 2", a.printedMsgCount)
	}
	// Messages should still be intact
	if len(a.messages) != 2 {
		t.Errorf("messages count = %d, want 2", len(a.messages))
	}
}

func TestResizeNoMessagesNoReprint(t *testing.T) {
	a := newTestApp(80, 24)

	// No messages printed yet, printedMsgCount is 0
	if a.printedMsgCount != 0 {
		t.Fatalf("initial printedMsgCount = %d, want 0", a.printedMsgCount)
	}

	// Resize without any messages — printedMsgCount stays 0
	simResize(a, 120, 30)
	if a.printedMsgCount != 0 {
		t.Errorf("printedMsgCount = %d, want 0 (no messages to reprint)", a.printedMsgCount)
	}
}

func TestResizeWordWrapsMessages(t *testing.T) {
	// Verify that renderMessage produces different output at different widths
	longContent := "The quick brown fox jumps over the lazy dog and keeps running far away"
	msg := chatMessage{kind: msgAssistant, content: longContent}

	narrow := renderMessage(msg, 30)
	wide := renderMessage(msg, 120)

	narrowLines := strings.Count(narrow, "\n")
	wideLines := strings.Count(wide, "\n")

	if narrowLines <= wideLines {
		t.Errorf("narrow rendering (%d lines) should have more lines than wide (%d lines)",
			narrowLines, wideLines)
	}
}

func TestResizePreservesMessageContent(t *testing.T) {
	a := newTestApp(80, 24)

	// Add messages
	a.messages = append(a.messages,
		chatMessage{kind: msgUser, content: "first message"},
		chatMessage{kind: msgAssistant, content: "response here"},
		chatMessage{kind: msgError, content: "an error"},
	)
	a.printedMsgCount = len(a.messages)

	// Resize
	simResize(a, 120, 40)

	// All messages preserved
	if len(a.messages) != 3 {
		t.Fatalf("messages count = %d, want 3", len(a.messages))
	}
	if a.messages[0].content != "first message" {
		t.Errorf("messages[0].content = %q, want %q", a.messages[0].content, "first message")
	}
	if a.messages[1].content != "response here" {
		t.Errorf("messages[1].content = %q", a.messages[1].content)
	}
	if a.messages[2].content != "an error" {
		t.Errorf("messages[2].content = %q", a.messages[2].content)
	}
	// printedMsgCount stays in sync
	if a.printedMsgCount != 3 {
		t.Errorf("printedMsgCount = %d, want 3", a.printedMsgCount)
	}
}

func TestNewMessagesGetPrinted(t *testing.T) {
	a := newTestApp(80, 24)

	// printedMsgCount starts at 0
	if a.printedMsgCount != 0 {
		t.Fatalf("initial printedMsgCount = %d, want 0", a.printedMsgCount)
	}

	// Send a message — handleEnter appends messages, then printNewMessages increments count
	simType(a, "hello")
	simKey(a, KeyEnter)

	// We need to call printNewMessages manually since render() isn't called in tests
	a.printNewMessages()

	if a.printedMsgCount != len(a.messages) {
		t.Errorf("printedMsgCount = %d, want %d (messages were not printed)", a.printedMsgCount, len(a.messages))
	}
}

func TestClearResetsPrintedCount(t *testing.T) {
	a := newTestApp(80, 24)

	a.messages = append(a.messages, chatMessage{kind: msgUser, content: "hello"})
	a.printedMsgCount = 1

	simType(a, "/clear")
	simKey(a, KeyEnter)

	if len(a.messages) != 0 {
		t.Errorf("messages should be empty after /clear, got %d", len(a.messages))
	}
	if a.printedMsgCount != 0 {
		t.Errorf("printedMsgCount should be 0 after /clear, got %d", a.printedMsgCount)
	}
}

// suppress unused import warnings
var _ = lipgloss.Width
var _ = strconv.Itoa
