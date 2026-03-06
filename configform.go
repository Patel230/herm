package main

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

type configField struct {
	label string
	desc  string
	input *TextInput
	err   string
}

type configForm struct {
	fields  []configField
	focused int
	width   int
	height  int
}

func newConfigForm(cfg Config, width, height int) configForm {
	pasteInput := NewTextInput(false)
	pasteInput.SetValue(strconv.Itoa(cfg.PasteCollapseMinChars))
	pasteInput.SetWidth(20)
	pasteInput.Focus()

	return configForm{
		fields: []configField{
			{label: "Paste collapse min chars", desc: "minimum characters to trigger paste collapsing", input: pasteInput},
			{label: "Anthropic API Key", desc: "key for Claude models", input: newAPIKeyInput(cfg.AnthropicAPIKey)},
			{label: "Grok API Key", desc: "key for Grok models", input: newAPIKeyInput(cfg.GrokAPIKey)},
			{label: "OpenAI API Key", desc: "key for GPT models", input: newAPIKeyInput(cfg.OpenAIAPIKey)},
			{label: "Gemini API Key", desc: "key for Gemini models", input: newAPIKeyInput(cfg.GeminiAPIKey)},
		},
		width:  width,
		height: height,
	}
}

// newAPIKeyInput creates a masked textinput for an API key field.
func newAPIKeyInput(value string) *TextInput {
	ti := NewTextInput(false)
	ti.SetValue(value)
	ti.echoMode = EchoPassword
	ti.Blur()
	return ti
}

// HandleKey processes a key event for the config form.
// Returns true if the form consumed the key.
func (f *configForm) HandleKey(key EventKey) bool {
	switch key.Key {
	case KeyRune:
		if key.Mod&ModCtrl != 0 {
			// Let ctrl+c pass through to caller
			return false
		}
		// Forward to focused input
		f.fields[f.focused].input.HandleKey(key)
		return true

	case KeyTab:
		if key.Mod&ModShift != 0 {
			f.fields[f.focused].input.Blur()
			f.focused = (f.focused - 1 + len(f.fields)) % len(f.fields)
			f.fields[f.focused].input.Focus()
		} else {
			f.fields[f.focused].input.Blur()
			f.focused = (f.focused + 1) % len(f.fields)
			f.fields[f.focused].input.Focus()
		}
		return true

	case KeyDown:
		f.fields[f.focused].input.Blur()
		f.focused = (f.focused + 1) % len(f.fields)
		f.fields[f.focused].input.Focus()
		return true

	case KeyUp:
		f.fields[f.focused].input.Blur()
		f.focused = (f.focused - 1 + len(f.fields)) % len(f.fields)
		f.fields[f.focused].input.Focus()
		return true

	default:
		// Forward other keys (backspace, delete, arrows, etc.) to focused input
		return f.fields[f.focused].input.HandleKey(key)
	}
}

// validate checks all fields and returns true if valid.
func (f *configForm) validate() bool {
	valid := true
	val := strings.TrimSpace(f.fields[0].input.Value())
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		f.fields[0].err = "Must be a positive integer"
		valid = false
	} else {
		f.fields[0].err = ""
	}
	return valid
}

// applyTo writes the form values into the given config.
func (f configForm) applyTo(cfg *Config) {
	n, _ := strconv.Atoi(strings.TrimSpace(f.fields[0].input.Value()))
	cfg.PasteCollapseMinChars = n
	cfg.AnthropicAPIKey = strings.TrimSpace(f.fields[1].input.Value())
	cfg.GrokAPIKey = strings.TrimSpace(f.fields[2].input.Value())
	cfg.OpenAIAPIKey = strings.TrimSpace(f.fields[3].input.Value())
	cfg.GeminiAPIKey = strings.TrimSpace(f.fields[4].input.Value())
}

func (f configForm) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#B88AFF")).
		Bold(true).
		PaddingLeft(2).
		PaddingBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9B6ADE")).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true)

	errStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B6B")).
		PaddingLeft(4)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555")).
		PaddingLeft(2).
		PaddingTop(1)

	hintKeyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7B3EC7"))

	promptFocused := lipgloss.NewStyle().Foreground(lipgloss.Color("#9B82F5")).Render("  ")
	promptBlurred := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render("  ")

	var b strings.Builder
	b.WriteString(titleStyle.Render("⚙ Configuration"))
	b.WriteString("\n")

	for i, field := range f.fields {
		cursor := lipgloss.NewStyle().Foreground(lipgloss.Color("#3A0066")).Render("  ")
		if i == f.focused {
			cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#B88AFF")).Render("▸ ")
		}
		b.WriteString(cursor)
		b.WriteString(labelStyle.Render(field.label))
		if field.desc != "" {
			b.WriteString("  ")
			b.WriteString(descStyle.Render(field.desc))
		}
		b.WriteString("\n")

		// Render the input value with prompt
		prompt := promptBlurred
		if i == f.focused {
			prompt = promptFocused
		}
		b.WriteString(fmt.Sprintf("    %s%s\n", prompt, field.input.View()))

		if field.err != "" {
			b.WriteString(errStyle.Render(field.err))
			b.WriteString("\n")
		}
	}

	hint := fmt.Sprintf(
		"  %s save  %s cancel",
		hintKeyStyle.Render("enter"),
		hintKeyStyle.Render("esc"),
	)
	b.WriteString(hintStyle.Render(hint))
	b.WriteString("\n")

	return b.String()
}
