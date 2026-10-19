// Package picker provides a standalone terminal project selector.
package picker

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// ErrCancelled indicates that the human dismissed the picker.
var ErrCancelled = errors.New("project selection cancelled")

// Item is a project available for selection.
type Item struct{ Key, Name string }

// Model is independent of the application's main TUI model.
type Model struct {
	items, matches    []Item
	filter            textinput.Model
	cursor, top, rows int
	selected          string
	err               error
	done              bool
}

// New constructs a picker with a focused filter and a bounded default window.
func New(items []Item) Model {
	input := textinput.New()
	input.Prompt = "Filter: "
	input.Placeholder = "project key or name"
	input.Focus()
	copied := append([]Item(nil), items...)
	return Model{items: copied, matches: copied, filter: input, rows: 10}
}

// Selected returns the chosen key, or an empty string before selection.
func (m Model) Selected() string { return m.selected }

// Err returns ErrCancelled when the picker was dismissed.
func (m Model) Err() error { return m.err }

// Init starts the filter cursor.
func (m Model) Init() tea.Cmd { return textinput.Blink }

// Update handles filtering, clamped navigation, selection and terminal resize.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.rows = max(1, min(20, msg.Height-4))
		m.filter.SetWidth(max(1, msg.Width-10))
		m.clamp()
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.err, m.done = ErrCancelled, true
			return m, tea.Quit
		case "enter":
			if len(m.matches) > 0 {
				m.selected, m.done = m.matches[m.cursor].Key, true
				return m, tea.Quit
			}
			return m, nil
		case "up", "down", "j", "k":
			if msg.String() == "up" || msg.String() == "down" || m.filter.Value() == "" {
				if msg.String() == "up" || msg.String() == "k" {
					m.cursor--
				} else {
					m.cursor++
				}
				m.clamp()
				return m, nil
			}
		}
	}
	previous := m.filter.Value()
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	if previous != m.filter.Value() {
		query := strings.ToLower(m.filter.Value())
		m.matches = nil
		for _, item := range m.items {
			if strings.Contains(strings.ToLower(item.Key), query) || strings.Contains(strings.ToLower(item.Name), query) {
				m.matches = append(m.matches, item)
			}
		}
		m.clamp()
	}
	return m, cmd
}

func (m *Model) clamp() {
	m.cursor = max(0, min(m.cursor, len(m.matches)-1))
	m.top = max(0, min(m.top, len(m.matches)-m.rows))
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+m.rows {
		m.top = m.cursor - m.rows + 1
	}
}

// View uses an explicit marker so selection never depends on color.
func (m Model) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	var b strings.Builder
	b.WriteString(m.filter.View() + "\n")
	if len(m.matches) == 0 {
		b.WriteString("No matching projects\n")
	}
	for i := m.top; i < min(len(m.matches), m.top+m.rows); i++ {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		// Strip control characters to keep each project on exactly one terminal row.
		clean := func(s string) string {
			return strings.Map(func(r rune) rune {
				if r < 32 || r == 127 {
					return -1
				}
				return r
			}, s)
		}
		fmt.Fprintf(&b, "%s%s  %s\n", marker, clean(m.matches[i].Key), clean(m.matches[i].Name))
	}
	position := 0
	if len(m.matches) > 0 {
		position = m.cursor + 1
	}
	fmt.Fprintf(&b, "%d/%d\n↑/↓ move · j/k move when filter empty · type to filter · Enter select · Esc cancel", position, len(m.matches))
	return tea.NewView(b.String())
}

// Pick runs on the caller's terminal; callers must gate use on terminal stdin/stdout.
func Pick(items []Item) (string, error) {
	if len(items) == 0 {
		return "", errors.New("no projects available to select")
	}
	// Keep stdout available for structured caller output; Bubble Tea still reads stdin.
	result, err := tea.NewProgram(New(items), tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return "", err
	}
	model := result.(Model)
	return model.Selected(), model.Err()
}
