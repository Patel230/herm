package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

type model struct {
	keys []string
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		s := fmt.Sprintf("String=%q Keystroke=%q", msg.String(), msg.Keystroke())
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m.keys = append(m.keys, s)
		if len(m.keys) > 20 {
			m.keys = m.keys[len(m.keys)-20:]
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	out := "Press keys to see their representation (ctrl+c to quit):\n\n"
	for _, k := range m.keys {
		out += k + "\n"
	}
	return tea.NewView(out)
}

func main() {
	p := tea.NewProgram(model{})
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
