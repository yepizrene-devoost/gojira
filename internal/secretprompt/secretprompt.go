// Package secretprompt reads a secret from a terminal without ever displaying it.
//
// Every typed or pasted character is replaced by a mask character, so the user can
// see that the input arrived but nobody looking at the screen, a recording or the
// scrollback can read it.
package secretprompt

import (
	"errors"
	"io"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// ErrCancelled is returned when the user dismisses the prompt with Esc or Ctrl+C.
var ErrCancelled = errors.New("cancelled")

type model struct {
	prompt    string
	input     textinput.Model
	value     string
	cancelled bool
	done      bool
}

func newModel(prompt string) model {
	input := textinput.New()
	input.EchoMode = textinput.EchoPassword
	input.EchoCharacter = '*'
	// No limit: Atlassian documents API tokens of variable length, and silently
	// truncating one would store a token that can never authenticate.
	input.CharLimit = 0
	// The label is drawn by the model so the mask starts right after it.
	input.Prompt = ""
	input.Focus()
	return model{prompt: prompt, input: input}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "enter":
			m.value = strings.TrimSpace(m.input.Value())
			m.done = true
			return m, tea.Quit
		case "esc", "ctrl+c":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) View() tea.View {
	content := m.prompt + m.input.View()
	if m.done {
		// Leave the cursor on a fresh line for whatever the caller prints next.
		content += "\n"
	}
	return tea.NewView(content)
}

// result is the entered secret, trimmed; it is empty when the prompt was cancelled.
func (m model) result() string { return m.value }

func (m model) wasCancelled() bool { return m.cancelled }

// Read shows prompt on out and reads one masked line from in. It returns the line
// trimmed of surrounding whitespace, or ErrCancelled when the user presses Esc or
// Ctrl+C. in must be a terminal for the masking to be meaningful.
func Read(prompt string, in io.Reader, out io.Writer) (string, error) {
	final, err := tea.NewProgram(newModel(prompt), tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return "", err
	}
	result, ok := final.(model)
	if !ok {
		return "", errors.New("secretprompt: unexpected final model")
	}
	if result.wasCancelled() {
		return "", ErrCancelled
	}
	return result.result(), nil
}
