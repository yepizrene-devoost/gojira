package secretprompt

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

const token = "ATATT3xFfGF0-very-secret-token-0123456789"

func send(t *testing.T, m model, msg tea.Msg) (model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	updated, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	return updated, cmd
}

func typeText(t *testing.T, m model, text string) model {
	t.Helper()
	for _, r := range text {
		m, _ = send(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

func TestPastedTokenIsNeverDisplayed(t *testing.T) {
	m := newModel("Jira API token: ")
	m, _ = send(t, m, tea.PasteMsg{Content: token})

	view := m.View().Content
	if strings.Contains(view, token) {
		t.Fatalf("the pasted token is visible: %q", view)
	}
	if strings.Contains(view, token[:6]) {
		t.Fatalf("part of the pasted token is visible: %q", view)
	}
	if got := strings.Count(view, "*"); got != len(token) {
		t.Fatalf("want %d mask characters so the paste is visibly acknowledged, got %d in %q", len(token), got, view)
	}
	if !strings.Contains(view, "Jira API token: ") {
		t.Fatalf("the prompt is missing from %q", view)
	}
}

func TestTypedTokenIsNeverDisplayed(t *testing.T) {
	m := typeText(t, newModel("Jira API token: "), token)

	view := m.View().Content
	if strings.Contains(view, token) || strings.Contains(view, token[:6]) {
		t.Fatalf("the typed token is visible: %q", view)
	}
	if got := strings.Count(view, "*"); got != len(token) {
		t.Fatalf("want %d mask characters, got %d", len(token), got)
	}
}

func TestEnterReturnsTheTokenTrimmed(t *testing.T) {
	m := newModel("Jira API token: ")
	m, _ = send(t, m, tea.PasteMsg{Content: token + "\n"})
	m, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Enter must quit the program")
	}
	if got := m.result(); got != token {
		t.Fatalf("result = %q, want the trimmed token", got)
	}
	if m.wasCancelled() {
		t.Fatal("Enter is not a cancellation")
	}
}

func TestEscapeAndCtrlCCancelWithoutReturningTheToken(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{
		{Code: tea.KeyEscape},
		{Code: 'c', Mod: tea.ModCtrl},
	} {
		t.Run(key.String(), func(t *testing.T) {
			m := newModel("Jira API token: ")
			m, _ = send(t, m, tea.PasteMsg{Content: token})
			m, cmd := send(t, m, key)

			if cmd == nil {
				t.Fatal("the key must quit the program")
			}
			if !m.wasCancelled() {
				t.Fatal("the key must cancel")
			}
			if m.result() != "" {
				t.Fatalf("a cancelled prompt must not return the token, got %q", m.result())
			}
		})
	}
}

func TestLongTokensAreNotTruncated(t *testing.T) {
	long := strings.Repeat("A", 600)
	m := newModel("Jira API token: ")
	m, _ = send(t, m, tea.PasteMsg{Content: long})
	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := len(m.result()); got != len(long) {
		t.Fatalf("a %d character token came back with %d characters", len(long), got)
	}
}
