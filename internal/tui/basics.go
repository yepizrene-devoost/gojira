package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/yepizrene-devoost/gojira/internal/issuefields"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// basicsLabels name the three inputs of the summary, priority and labels form.
var basicsLabels = [3]string{"Summary", "Priority", "Labels"}

// fieldProblem is a validation message shown to the user as typed here.
type fieldProblem string

func (p fieldProblem) Error() string { return string(p) }

type basicsState struct {
	inputs   [3]textinput.Model
	original [3]string
	cur      int
	key      string
	// request, board, search and detail scope replies to the form that sent them.
	request, board, search, detail uint64
	busy, applied                  bool
	err                            error
}

type basicsUpdatedMsg struct {
	request uint64
	key     string
	issue   *jira.Issue
	applied bool
	err     error
}

// basicsValues are the form values for a ticket: summary, priority name, labels.
func basicsValues(issue jira.Issue) [3]string {
	values := [3]string{issue.Fields.Summary, "", strings.Join(issue.Fields.Labels, ",")}
	if issue.Fields.Priority != nil {
		values[1] = issue.Fields.Priority.Name
	}
	return values
}

func (m Model) startBasicsEdit(issue jira.Issue) (tea.Model, tea.Cmd) {
	state := basicsState{
		key: issue.Key, original: basicsValues(issue), request: m.basics.request + 1,
		board: m.loadedBoardRequest, search: m.activeSearchRequest, detail: m.activeDetailRequest,
	}
	for i := range state.inputs {
		input := textinput.New()
		input.Prompt = ""
		input.SetWidth(max(m.modalWidth()-20, 12))
		// No limit: Jira reports an over-long summary itself, and silently truncating
		// what the user typed would be worse.
		input.CharLimit = 0
		input.SetValue(state.original[i])
		state.inputs[i] = input
	}
	state.inputs[0].Focus()
	m.basics = state
	m.err = nil
	m.view = viewBasicsEdit
	return m, textinput.Blink
}

func (m Model) matchBasics(msg basicsUpdatedMsg) bool {
	b := m.basics
	return m.view == viewBasicsEdit && b.busy && b.request == msg.request && m.detail != nil &&
		m.detail.Key == b.key && msg.key == b.key && b.board == m.loadedBoardRequest &&
		b.search == m.activeSearchRequest && b.detail == m.activeDetailRequest && m.activeBoardRequest == 0
}

func (m Model) updateBasicsEdit(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		// Leaving invalidates replies, not a request that is already on its way.
		applied := m.basics.applied
		m.basics.request++
		m.basics.busy, m.basics.applied = false, false
		m.view = viewDetail
		if applied {
			m.toast = "Update applied; detail refresh failed"
			return m, clearToastAfter(3 * time.Second)
		}
		return m, nil
	case "ctrl+s":
		if m.basics.busy {
			return m, nil
		}
		return m.startBasicsSave(m.basics.applied)
	case "tab", "shift+tab":
		if m.basics.busy || m.basics.applied {
			return m, nil
		}
		m.basics.inputs[m.basics.cur].Blur()
		step := 1
		if key == "shift+tab" {
			step = len(m.basics.inputs) - 1
		}
		m.basics.cur = (m.basics.cur + step) % len(m.basics.inputs)
		return m, m.basics.inputs[m.basics.cur].Focus()
	}
	if m.basics.busy || m.basics.applied {
		return m, nil
	}
	var cmd tea.Cmd
	m.basics.inputs[m.basics.cur], cmd = m.basics.inputs[m.basics.cur].Update(msg)
	return m, cmd
}

func (m Model) pasteBasics(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	if m.basics.busy || m.basics.applied {
		return m, nil
	}
	var cmd tea.Cmd
	m.basics.inputs[m.basics.cur], cmd = m.basics.inputs[m.basics.cur].Update(msg)
	return m, cmd
}

// preparedBasics builds the update from the changed inputs only, through the same shared
// preparation `gojira update` uses. The boolean reports whether anything changed.
func (m Model) preparedBasics() (issuefields.Prepared, bool, error) {
	var in issuefields.Input
	changed := false
	summary := strings.TrimSpace(m.basics.inputs[0].Value())
	if summary != m.basics.original[0] {
		if summary == "" {
			return issuefields.Prepared{}, false, fieldProblem("Summary cannot be empty")
		}
		in.Summary, changed = &summary, true
	}
	priority := strings.TrimSpace(m.basics.inputs[1].Value())
	if priority != m.basics.original[1] {
		if priority == "" {
			return issuefields.Prepared{}, false, fieldProblem("Priority cannot be empty")
		}
		in.Priority, changed = &priority, true
	}
	if raw := m.basics.inputs[2].Value(); strings.TrimSpace(raw) == "" {
		if m.basics.original[2] != "" {
			return issuefields.Prepared{}, false, fieldProblem("Labels cannot be cleared; keep at least one")
		}
	} else {
		names, err := issuefields.ParseLabelList(raw)
		if err != nil {
			return issuefields.Prepared{}, false, err
		}
		if strings.Join(names, ",") != m.basics.original[2] {
			in.LabelNames, changed = &names, true
		}
	}
	if !changed {
		return issuefields.Prepared{}, false, nil
	}
	prepared, err := issuefields.Prepare(in)
	return prepared, err == nil, err
}

func (m Model) startBasicsSave(refreshOnly bool) (tea.Model, tea.Cmd) {
	var fields map[string]interface{}
	if !refreshOnly {
		prepared, changed, err := m.preparedBasics()
		if err != nil {
			m.basics.err = err
			return m, nil
		}
		if !changed {
			m.basics.err = nil
			m.view = viewDetail
			return m, nil
		}
		fields = prepared.Fields
	}
	m.basics.err = nil
	m.basics.busy = true
	client, key, request := m.client, m.basics.key, m.basics.request
	return m, func() tea.Msg {
		result := basicsUpdatedMsg{request: request, key: key, applied: refreshOnly}
		if !refreshOnly {
			if err := client.UpdateIssue(key, fields); err != nil {
				result.err = err
				return result
			}
			result.applied = true
		}
		result.issue, result.err = client.GetIssueFull(key)
		return result
	}
}

func (m Model) basicsUpdated(msg basicsUpdatedMsg) (tea.Model, tea.Cmd) {
	if !m.matchBasics(msg) {
		return m, nil
	}
	m.basics.busy = false
	if msg.err != nil {
		m.basics.applied, m.basics.err = msg.applied, msg.err
		return m, nil
	}
	if msg.issue == nil || msg.issue.Key != msg.key {
		m.basics.applied = msg.applied
		m.basics.err = fieldProblem("update applied, but the refreshed ticket is missing or invalid")
		return m, nil
	}
	m.detail = msg.issue
	m = m.withEditedFields(*msg.issue)
	m.detailScroll = 0
	m.basics.applied, m.basics.err = false, nil
	m.view = viewDetail
	m.toast = "Ticket fields updated"
	return m, clearToastAfter(3 * time.Second)
}

func (m Model) renderBasicsEdit() string {
	var b strings.Builder
	b.WriteString(keyStyle.Render("✎ Edit summary, priority and labels · "+m.basics.key) + "\n\n")
	for i, label := range basicsLabels {
		marker := "  "
		if i == m.basics.cur {
			marker = "→ "
		}
		b.WriteString(marker + label + ": " + m.basics.inputs[i].View() + "\n")
	}
	b.WriteString("\nLabels are comma-separated. Summary and priority cannot be empty, and labels cannot be cleared.\n")
	if m.basics.busy {
		b.WriteString("Updating ticket...\n")
	}
	if m.basics.applied {
		b.WriteString("Update applied; retry the refresh only with Ctrl+S.\n")
	}
	if m.basics.err != nil {
		b.WriteString(errStyle.Render("Error: "+m.basics.err.Error()) + "\n")
	}
	b.WriteString("[tab] Next  [ctrl+s] Save  [esc] Cancel")
	return m.modalStyle().Render(b.String())
}

// withEditedFields copies the edited summary, priority and labels of a refreshed ticket
// onto its board card and search row, leaving every other field and ticket as it was.
// The lists are copied, never changed in place: earlier copies of the model share them.
func (m Model) withEditedFields(issue jira.Issue) Model {
	edit := func(card jira.Issue) jira.Issue {
		if card.Key == issue.Key {
			card.Fields.Summary = issue.Fields.Summary
			card.Fields.Priority = issue.Fields.Priority
			card.Fields.Labels = issue.Fields.Labels
		}
		return card
	}
	editAll := func(cards []jira.Issue) []jira.Issue {
		edited := make([]jira.Issue, len(cards))
		for i, card := range cards {
			edited[i] = edit(card)
		}
		return edited
	}
	columns := make([]column, len(m.columns))
	for i, col := range m.columns {
		col.issues = editAll(col.issues)
		columns[i] = col
	}
	m.columns = columns
	m.searchResults = editAll(m.searchResults)
	return m
}
