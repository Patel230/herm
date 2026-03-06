package main

import "testing"

func TestTextInputBasic(t *testing.T) {
	ti := NewTextInput(true)
	if ti.Value() != "" {
		t.Errorf("new input should be empty, got %q", ti.Value())
	}

	ti.InsertRune('h')
	ti.InsertRune('i')
	if ti.Value() != "hi" {
		t.Errorf("expected %q, got %q", "hi", ti.Value())
	}

	ti.Backspace()
	if ti.Value() != "h" {
		t.Errorf("after backspace expected %q, got %q", "h", ti.Value())
	}
}

func TestTextInputMultiLine(t *testing.T) {
	ti := NewTextInput(true)
	ti.InsertText("hello")
	ti.InsertNewline()
	ti.InsertText("world")
	if ti.Value() != "hello\nworld" {
		t.Errorf("expected %q, got %q", "hello\nworld", ti.Value())
	}
	if ti.LineCount() != 2 {
		t.Errorf("expected 2 lines, got %d", ti.LineCount())
	}
}

func TestTextInputSingleLineNoNewline(t *testing.T) {
	ti := NewTextInput(false)
	ti.InsertText("hello")
	ti.InsertNewline() // should be no-op
	ti.InsertText("world")
	if ti.Value() != "helloworld" {
		t.Errorf("expected %q, got %q", "helloworld", ti.Value())
	}
}

func TestTextInputSetValue(t *testing.T) {
	ti := NewTextInput(true)
	ti.SetValue("line1\nline2\nline3")
	if ti.LineCount() != 3 {
		t.Errorf("expected 3 lines, got %d", ti.LineCount())
	}
	if ti.cursorRow != 2 || ti.cursorCol != 5 {
		t.Errorf("cursor should be at end (2,5), got (%d,%d)", ti.cursorRow, ti.cursorCol)
	}
}

func TestTextInputReset(t *testing.T) {
	ti := NewTextInput(true)
	ti.SetValue("some content")
	ti.Reset()
	if ti.Value() != "" {
		t.Errorf("after reset expected empty, got %q", ti.Value())
	}
}

func TestTextInputCursorMovement(t *testing.T) {
	ti := NewTextInput(true)
	ti.SetValue("abc\ndef")
	// Cursor is at end: row 1, col 3
	ti.MoveHome()
	if ti.cursorCol != 0 {
		t.Errorf("MoveHome: col should be 0, got %d", ti.cursorCol)
	}
	ti.MoveEnd()
	if ti.cursorCol != 3 {
		t.Errorf("MoveEnd: col should be 3, got %d", ti.cursorCol)
	}
	ti.MoveUp()
	if ti.cursorRow != 0 {
		t.Errorf("MoveUp: row should be 0, got %d", ti.cursorRow)
	}
	ti.MoveDown()
	if ti.cursorRow != 1 {
		t.Errorf("MoveDown: row should be 1, got %d", ti.cursorRow)
	}
}

func TestTextInputWordBoundaries(t *testing.T) {
	ti := NewTextInput(false)
	ti.SetValue("hello world test")
	// Cursor at end (col 16)
	ti.MoveWordLeft() // should go to start of "test" (col 12)
	if ti.cursorCol != 12 {
		t.Errorf("MoveWordLeft: expected col 12, got %d", ti.cursorCol)
	}
	ti.MoveWordLeft() // should go to start of "world" (col 6)
	if ti.cursorCol != 6 {
		t.Errorf("MoveWordLeft: expected col 6, got %d", ti.cursorCol)
	}
	ti.MoveWordRight() // should go past "world " to start of "test" (col 12)
	if ti.cursorCol != 12 {
		t.Errorf("MoveWordRight: expected col 12, got %d", ti.cursorCol)
	}
}

func TestTextInputDeleteWordBackward(t *testing.T) {
	ti := NewTextInput(false)
	ti.SetValue("hello world")
	ti.DeleteWordBackward()
	if ti.Value() != "hello " {
		t.Errorf("expected %q, got %q", "hello ", ti.Value())
	}
}

func TestTextInputKillLine(t *testing.T) {
	ti := NewTextInput(false)
	ti.SetValue("hello world")
	ti.MoveHome()
	ti.MoveWordRight() // cursor after "hello "
	ti.KillLine()
	if ti.Value() != "hello " {
		t.Errorf("expected %q, got %q", "hello ", ti.Value())
	}
}

func TestTextInputDelete(t *testing.T) {
	ti := NewTextInput(false)
	ti.SetValue("abc")
	ti.MoveHome()
	ti.Delete()
	if ti.Value() != "bc" {
		t.Errorf("expected %q, got %q", "bc", ti.Value())
	}
}

func TestTextInputPasteMultiLine(t *testing.T) {
	ti := NewTextInput(true)
	ti.InsertText("start")
	ti.InsertText("line1\nline2\nline3")
	if ti.Value() != "startline1\nline2\nline3" {
		t.Errorf("expected %q, got %q", "startline1\nline2\nline3", ti.Value())
	}
	if ti.cursorRow != 2 || ti.cursorCol != 5 {
		t.Errorf("cursor should be at (2,5), got (%d,%d)", ti.cursorRow, ti.cursorCol)
	}
}

func TestTextInputPasteMultiLineSingleMode(t *testing.T) {
	ti := NewTextInput(false)
	ti.InsertText("a\nb\nc")
	if ti.Value() != "a b c" {
		t.Errorf("single-line paste should join with spaces, got %q", ti.Value())
	}
}

func TestTextInputCursorPosition(t *testing.T) {
	ti := NewTextInput(true)
	ti.SetWidth(80)
	ti.SetValue("hello\nworld")
	// Cursor at end of "world" (row 1, col 5)
	x, y := ti.CursorPosition()
	if x != 5 || y != 1 {
		t.Errorf("expected cursor (5,1), got (%d,%d)", x, y)
	}
}

func TestTextInputPasswordMode(t *testing.T) {
	ti := NewTextInput(false)
	ti.echoMode = EchoPassword
	ti.SetValue("secret")
	view := ti.View()
	if view != "******" {
		t.Errorf("password view should be asterisks, got %q", view)
	}
}

func TestTextInputBackspaceAtLineStart(t *testing.T) {
	ti := NewTextInput(true)
	ti.SetValue("ab\ncd")
	ti.cursorRow = 1
	ti.cursorCol = 0
	ti.Backspace()
	if ti.Value() != "abcd" {
		t.Errorf("expected %q, got %q", "abcd", ti.Value())
	}
	if ti.cursorRow != 0 || ti.cursorCol != 2 {
		t.Errorf("cursor should be at (0,2), got (%d,%d)", ti.cursorRow, ti.cursorCol)
	}
}

func TestTextInputDeleteAtLineEnd(t *testing.T) {
	ti := NewTextInput(true)
	ti.SetValue("ab\ncd")
	ti.cursorRow = 0
	ti.cursorCol = 2
	ti.Delete()
	if ti.Value() != "abcd" {
		t.Errorf("expected %q, got %q", "abcd", ti.Value())
	}
}

func TestTextInputDisplayLineCount(t *testing.T) {
	ti := NewTextInput(true)
	ti.SetWidth(5)
	ti.SetValue("abcdefghij") // 10 chars, width 5 -> 2 visual lines
	if c := ti.DisplayLineCount(); c != 2 {
		t.Errorf("expected 2 display lines, got %d", c)
	}
}
