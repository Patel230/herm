package main

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

type configField struct {
	label string
	input textinput.Model
	err   string
}

type configForm struct {
	fields  []configField
	focused int
	width   int
	height  int
}

func newConfigForm(cfg Config, width, height int) configForm {
	pasteInput := textinput.New()
	pasteInput.Placeholder = "200"
	pasteInput.SetValue(strconv.Itoa(cfg.PasteCollapseMinChars))
	pasteInput.Focus()
	pasteInput.CharLimit = 10
	pasteInput.SetWidth(20)

	return configForm{
		fields: []configField{
			{label: "Paste collapse min chars", input: pasteInput},
		},
		width:  width,
		height: height,
	}
}

func (f configForm) Update(msg tea.Msg) (configForm, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		f.width = msg.Width
		f.height = msg.Height

	case tea.KeyPressMsg:
		switch msg.String() {
		case "tab", "down":
			f.fields[f.focused].input.Blur()
			f.focused = (f.focused + 1) % len(f.fields)
			return f, f.fields[f.focused].input.Focus()
		case "shift+tab", "up":
			f.fields[f.focused].input.Blur()
			f.focused = (f.focused - 1 + len(f.fields)) % len(f.fields)
			return f, f.fields[f.focused].input.Focus()
		}
	}

	var cmd tea.Cmd
	f.fields[f.focused].input, cmd = f.fields[f.focused].input.Update(msg)
	return f, cmd
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
}

func (f configForm) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#B88AFF")).
		Bold(true).
		PaddingLeft(2).
		PaddingBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9B6ADE")).
		PaddingLeft(2)

	errStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B6B")).
		PaddingLeft(4)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		PaddingLeft(2).
		PaddingTop(1)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Configuration"))
	b.WriteString("\n")

	for i, field := range f.fields {
		prefix := "  "
		if i == f.focused {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("#B88AFF")).Render("▸ ")
		}
		b.WriteString(prefix)
		b.WriteString(labelStyle.Render(field.label))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("    %s\n", field.input.View()))
		if field.err != "" {
			b.WriteString(errStyle.Render(field.err))
			b.WriteString("\n")
		}
	}

	b.WriteString(hintStyle.Render("enter: save • esc: cancel"))
	b.WriteString("\n")

	return b.String()
}
