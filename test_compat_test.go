package main

// Temporary compatibility helpers for test files that still use the old
// model-based API. These will be removed in tasks 7c/7d when the remaining
// test files are ported to the App-based helpers.

import (
	tea "charm.land/bubbletea/v2"
)

// keyPress creates a KeyPressMsg for a printable character.
func keyPress(key rune) tea.Msg {
	return tea.KeyPressMsg{Code: key, Text: string(key)}
}

// typeString feeds each rune of s into the model's Update loop.
func typeString(m model, s string) model {
	for _, r := range s {
		result, _ := m.Update(keyPress(r))
		m = result.(model)
	}
	return m
}

// sendKey feeds a single KeyPressMsg into the model.
func sendKey(m model, code rune, mods ...tea.KeyMod) model {
	msg := tea.KeyPressMsg{Code: code}
	for _, mod := range mods {
		msg.Mod |= mod
	}
	result, _ := m.Update(msg)
	return result.(model)
}

// resize sends a WindowSizeMsg and returns the updated model.
func resize(m model, w, h int) model {
	result, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return result.(model)
}

// paste simulates a bracketed paste event and returns the updated model.
func paste(m model, content string) model {
	result, _ := m.Update(tea.PasteMsg{Content: content})
	return result.(model)
}

// modelWithKey returns a model with only an Anthropic API key configured
// and test models loaded.
func modelWithKey() model {
	m := initialModel()
	m.config.AnthropicAPIKey = "sk-test-key"
	m.config.GrokAPIKey = ""
	m.config.OpenAIAPIKey = ""
	m.config.ActiveModel = ""
	m.models = testModels()
	m.modelsLoaded = true
	return m
}

// enterModelModeWith sets up a model with the given models and enters /model.
func enterModelModeWith(t interface{ Helper(); Fatalf(string, ...any) }, models []ModelDef) model {
	t.Helper()
	m := initialModel()
	m.config.AnthropicAPIKey = "key"
	m.config.OpenAIAPIKey = "key"
	m.config.GrokAPIKey = "key"
	m.models = models
	m.modelsLoaded = true
	m = resize(m, 120, 40)

	m = typeString(m, "/model")
	m = sendKey(m, tea.KeyEnter)
	if m.mode != modeModel {
		t.Fatalf("expected modeModel, got %d", m.mode)
	}
	return m
}

