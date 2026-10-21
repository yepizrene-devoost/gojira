package tui

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func basicsIssue() *jira.Issue {
	return &jira.Issue{Key: "TEST-1", Fields: jira.IssueFieldData{
		Summary:  "Old summary",
		Priority: &jira.PriorityField{Name: "Medium"},
		Labels:   []string{"backend", "urgent"},
	}}
}

// basicsModel is a Model showing TEST-1's detail, with every request counted.
func basicsModel(t *testing.T, origin view, handler http.HandlerFunc) (Model, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	m := assignmentModel(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		handler(w, r)
	})
	m.view = viewDetail
	m.detailReturn = origin
	m.detail = basicsIssue()
	return m, &requests
}

func basicsPress(m Model, name string) (Model, tea.Cmd) {
	var msg tea.KeyPressMsg
	switch name {
	case "tab":
		msg = tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		msg = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "ctrl+s":
		msg = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "ctrl+c":
		msg = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "backspace":
		msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "esc":
		msg = tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		msg = tea.KeyPressMsg{Code: []rune(name)[0], Text: name}
	}
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func basicsType(m Model, text string) Model {
	for _, r := range text {
		m, _ = basicsPress(m, string(r))
	}
	return m
}

func noRequests(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
	}
}

func TestBasicsOpensFromTheDetailPrefilledWithTheCurrentValues(t *testing.T) {
	for name, origin := range map[string]view{"board": viewKanban, "search": viewSearch} {
		t.Run(name, func(t *testing.T) {
			m, requests := basicsModel(t, origin, noRequests(t))
			m, _ = basicsPress(m, "U")
			content := m.View().Content
			for _, want := range []string{"Edit summary, priority and labels", "TEST-1", "Old summary", "Medium", "backend,urgent", "→ Summary"} {
				if !strings.Contains(content, want) {
					t.Fatalf("the form is missing %q:\n%s", want, content)
				}
			}
			if requests.Load() != 0 {
				t.Fatalf("opening the form sent %d requests", requests.Load())
			}
		})
	}
}

func TestBasicsIgnoresTheKeyWithoutAUsableDetail(t *testing.T) {
	m, _ := basicsModel(t, viewKanban, noRequests(t))
	m.detail = nil
	m, _ = basicsPress(m, "U")
	if m.view != viewDetail {
		t.Fatalf("U changed the view without a ticket: %v", m.view)
	}
	m, _ = basicsModel(t, viewKanban, noRequests(t))
	m.detailLoading = true
	m, _ = basicsPress(m, "U")
	if strings.Contains(m.View().Content, "Edit summary, priority and labels") {
		t.Fatal("U opened the form while the detail was still loading")
	}
}

func TestBasicsTabMovesTheFocusAndTypingEditsTheFocusedInput(t *testing.T) {
	m, _ := basicsModel(t, viewKanban, noRequests(t))
	m, _ = basicsPress(m, "U")
	m, _ = basicsPress(m, "tab")
	if !strings.Contains(m.View().Content, "→ Priority") {
		t.Fatalf("Tab did not focus Priority:\n%s", m.View().Content)
	}
	m = basicsType(m, "X")
	if !strings.Contains(m.View().Content, "MediumX") {
		t.Fatalf("typing did not edit the focused input:\n%s", m.View().Content)
	}
	m, _ = basicsPress(m, "tab")
	if !strings.Contains(m.View().Content, "→ Labels") {
		t.Fatal("Tab did not focus Labels")
	}
	m, _ = basicsPress(m, "tab")
	if !strings.Contains(m.View().Content, "→ Summary") {
		t.Fatal("Tab did not wrap around to Summary")
	}
	m, _ = basicsPress(m, "shift+tab")
	if !strings.Contains(m.View().Content, "→ Labels") {
		t.Fatal("Shift+Tab did not go back to Labels")
	}
}

func TestBasicsPasteGoesIntoTheFocusedInput(t *testing.T) {
	m, _ := basicsModel(t, viewKanban, noRequests(t))
	m, _ = basicsPress(m, "U")
	next, _ := m.Update(tea.PasteMsg{Content: " pasted"})
	m = next.(Model)
	if !strings.Contains(m.View().Content, "Old summary pasted") {
		t.Fatalf("paste did not reach the summary:\n%s", m.View().Content)
	}
}

func TestBasicsEscCancelsWithoutAnyRequestAndKeepsTheTicket(t *testing.T) {
	m, requests := basicsModel(t, viewKanban, noRequests(t))
	m, _ = basicsPress(m, "U")
	m = basicsType(m, " edited")
	m, _ = basicsPress(m, "esc")
	if m.view != viewDetail || m.detail == nil || m.detail.Fields.Summary != "Old summary" || requests.Load() != 0 {
		t.Fatalf("cancel view=%v detail=%+v requests=%d", m.view, m.detail, requests.Load())
	}
	if strings.Contains(m.View().Content, "Edit summary, priority and labels") {
		t.Fatal("the form is still visible after Esc")
	}
}

func TestBasicsCtrlCQuitsFromTheForm(t *testing.T) {
	m, _ := basicsModel(t, viewKanban, noRequests(t))
	m, _ = basicsPress(m, "U")
	_, cmd := basicsPress(m, "ctrl+c")
	if cmd == nil {
		t.Fatal("Ctrl+C returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("Ctrl+C did not quit")
	}
}

// The existing detail flows keep their own keys next to the new one.
func TestBasicsDoesNotChangeTheExistingDetailKeys(t *testing.T) {
	for key, want := range map[string]string{"E": "Edit fields", "e": "Edit description", "A": "Assignee email"} {
		m, _ := basicsModel(t, viewKanban, noRequests(t))
		m, _ = basicsPress(m, key)
		if !strings.Contains(m.View().Content, want) {
			t.Errorf("%s no longer opens %q:\n%s", key, want, m.View().Content)
		}
	}
}

func TestBasicsHintsAreInTheDetailFooterAndTheHelp(t *testing.T) {
	m, _ := basicsModel(t, viewKanban, noRequests(t))
	if !strings.Contains(m.View().Content, "[U] Summary, priority, labels") {
		t.Fatalf("the detail footer does not mention U:\n%s", m.View().Content)
	}
	m.view = viewKanban
	m.detail = nil
	m, _ = basicsPress(m, "?")
	for _, want := range []string{"U (detail)", "Edit summary, priority and labels", "Assign ticket by email", "Add comment"} {
		if !strings.Contains(m.View().Content, want) {
			t.Fatalf("help is missing %q", want)
		}
	}
}
