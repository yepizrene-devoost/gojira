package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// ─── Setup wizard model ────────────────────────────────────────────────

type setupStep int

const (
	stepDomain setupStep = iota
	stepEmail
	stepToken
	stepTesting
	stepDone
	stepFailed
)

// SetupModel is a standalone Bubble Tea model for first-run configuration.
// It collects domain, email, and token, tests the connection, and saves to disk.
type SetupModel struct {
	step       setupStep
	domain     textinput.Model
	email      textinput.Model
	token      textinput.Model
	err        string
	testResult string
	width      int
	height     int
}

// NewSetup returns a SetupModel ready to run with tea.NewProgram.
func NewSetup() SetupModel {
	d := textinput.New()
	d.Placeholder = "your-domain.atlassian.net"
	d.Focus()
	d.CharLimit = 100

	e := textinput.New()
	e.Placeholder = "you@example.com"
	e.CharLimit = 100

	t := textinput.New()
	t.Placeholder = "Jira API token"
	t.EchoMode = textinput.EchoPassword
	t.CharLimit = 200

	return SetupModel{
		step:   stepDomain,
		domain: d,
		email:  e,
		token:  t,
	}
}

func (m SetupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			switch m.step {
			case stepDomain:
				if strings.TrimSpace(m.domain.Value()) == "" {
					return m, nil
				}
				m.step = stepEmail
				m.email.Focus()
				return m, textinput.Blink

			case stepEmail:
				if strings.TrimSpace(m.email.Value()) == "" {
					return m, nil
				}
				m.step = stepToken
				m.token.Focus()
				return m, textinput.Blink

			case stepToken:
				if strings.TrimSpace(m.token.Value()) == "" {
					return m, nil
				}
				m.step = stepTesting
				return m, m.testAndSave()

			case stepDone, stepFailed:
				return m, tea.Quit
			}

		case "esc":
			if m.step > stepDomain {
				switch m.step {
				case stepEmail:
					m.step = stepDomain
					m.domain.Focus()
				case stepToken:
					m.step = stepEmail
					m.email.Focus()
				}
				return m, nil
			}
		}
	}

	// Forward to active input
	var cmd tea.Cmd
	switch m.step {
	case stepDomain:
		m.domain, cmd = m.domain.Update(msg)
	case stepEmail:
		m.email, cmd = m.email.Update(msg)
	case stepToken:
		m.token, cmd = m.token.Update(msg)
	}
	return m, cmd
}

func (m SetupModel) testAndSave() tea.Cmd {
	return func() tea.Msg {
		domain := strings.TrimSpace(m.domain.Value())
		email := strings.TrimSpace(m.email.Value())
		token := strings.TrimSpace(m.token.Value())

		// Test connection
		client := jira.NewClient("https://"+domain, email, token)
		info, err := client.TestConnection()
		if err != nil {
			return setupDoneMsg{err: err}
		}

		// Save config
		cfg := &config.Config{Domain: domain, Email: email}
		if err := cfg.Save(); err != nil {
			return setupDoneMsg{err: fmt.Errorf("saving config: %w", err)}
		}
		if err := config.SaveToken(token); err != nil {
			return setupDoneMsg{err: fmt.Errorf("saving token: %w", err)}
		}

		return setupDoneMsg{info: info}
	}
}

type setupDoneMsg struct {
	info string
	err  error
}

func (m SetupModel) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).Render("🚀 GoJira Setup")
	b.WriteString(title)
	b.WriteString("\n\n")

	switch m.step {
	case stepDomain:
		b.WriteString("Step 1/3 — Your Jira domain\n\n")
		b.WriteString("  Enter your Atlassian domain (without https://)\n")
		b.WriteString("  Example: mycompany.atlassian.net\n\n")
		b.WriteString("  " + m.domain.View())
		b.WriteString("\n\n")
		b.WriteString(subStyle.Render("[Enter] Continue  [Ctrl+C] Quit"))

	case stepEmail:
		b.WriteString("Step 2/3 — Your email\n\n")
		b.WriteString("  The email you use to log into Jira\n\n")
		b.WriteString("  " + m.email.View())
		b.WriteString("\n\n")
		b.WriteString(subStyle.Render("[Enter] Continue  [Esc] Back  [Ctrl+C] Quit"))

	case stepToken:
		b.WriteString("Step 3/3 — API token\n\n")
		b.WriteString("  Create one at: https://id.atlassian.com/manage-profile/security/api-tokens\n\n")
		b.WriteString("  " + m.token.View())
		b.WriteString("\n\n")
		b.WriteString(subStyle.Render("[Enter] Test & Save  [Esc] Back  [Ctrl+C] Quit"))

	case stepTesting:
		b.WriteString(subStyle.Render("Testing connection..."))

	case stepDone:
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Bold(true).Render("✓ " + m.testResult))
		b.WriteString("\n\n")
		b.WriteString(subStyle.Render("Config saved to ~/.config/gojira/config.yaml"))
		b.WriteString("\n")
		b.WriteString(subStyle.Render("[Enter] Continue to GoJira"))

	case stepFailed:
		b.WriteString(errStyle.Render("✗ " + m.err))
		b.WriteString("\n\n")
		b.WriteString(subStyle.Render("[Enter] Edit token  [Ctrl+C] Quit"))
	}

	return b.String()
}
