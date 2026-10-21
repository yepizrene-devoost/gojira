package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

type assignmentState struct {
	input      textinput.Model
	key        string
	origin     view
	validation string
	request    uint64
	board      uint64
	search     uint64
	detail     uint64
	busy       bool
	finished   bool
	assigning  bool
}

type assignmentResolvedMsg struct {
	request uint64
	account string
	name    string
	err     error
}

type assignmentDoneMsg struct {
	request uint64
	name    string
	err     error
}

func (m Model) startAssignment(issue jira.Issue) (tea.Model, tea.Cmd) {
	if m.activeBoardRequest != 0 || issue.Key == "" {
		return m, nil
	}
	input := textinput.New()
	input.Prompt = "Assignee email: "
	input.CharLimit = 254
	input.SetWidth(min(40, max(m.width-22, 1)))
	input.Focus()
	m.assignment = assignmentState{
		input: input, key: issue.Key, origin: m.view,
		request: m.assignment.request + 1,
		board:   m.loadedBoardRequest, search: m.activeSearchRequest, detail: m.activeDetailRequest,
	}
	m.err = nil
	m.view = viewAssign
	return m, textinput.Blink
}

func (m Model) matchAssignment(request uint64) bool {
	a := m.assignment
	return m.view == viewAssign && a.busy && a.request == request &&
		a.board == m.loadedBoardRequest && a.search == m.activeSearchRequest &&
		a.detail == m.activeDetailRequest && m.activeBoardRequest == 0
}

func (m Model) updateAssignment(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		// Leaving invalidates replies, not an HTTP mutation already in flight.
		m.assignment.request++
		m.assignment.busy = false
		m.view = m.assignment.origin
		m.err = nil
		return m, nil
	case "enter":
		if m.assignment.busy || m.assignment.finished {
			return m, nil
		}
		email := strings.TrimSpace(m.assignment.input.Value())
		if email == "" {
			m.assignment.validation = "Email is required"
			return m, nil
		}
		m.assignment.validation = ""
		m.assignment.busy = true
		client, request := m.client, m.assignment.request
		return m, func() tea.Msg {
			account, name, err := client.ResolveAccountID(email)
			return assignmentResolvedMsg{request: request, account: account, name: name, err: err}
		}
	}
	if m.assignment.busy || m.assignment.finished {
		return m, nil
	}
	var cmd tea.Cmd
	m.assignment.input, cmd = m.assignment.input.Update(msg)
	return m, cmd
}

func (m Model) assignmentResolved(msg assignmentResolvedMsg) (tea.Model, tea.Cmd) {
	if !m.matchAssignment(msg.request) || m.assignment.assigning {
		return m, nil
	}
	if msg.err != nil {
		m.assignment.busy, m.assignment.finished = false, true
		m.err = msg.err
		return m, nil
	}
	m.assignment.assigning = true
	client, key := m.client, m.assignment.key
	return m, func() tea.Msg {
		return assignmentDoneMsg{request: msg.request, name: msg.name, err: client.AssignIssue(key, msg.account)}
	}
}

func (m Model) assignmentDone(msg assignmentDoneMsg) (tea.Model, tea.Cmd) {
	if !m.matchAssignment(msg.request) {
		return m, nil
	}
	m.assignment.busy, m.assignment.finished = false, true
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	m.toast = "Assigned " + m.assignment.key + " to " + msg.name
	m.detail = nil
	if m.assignment.origin == viewDetail && m.detailReturn == viewSearch {
		return m.startSearch(m.searchQuery)
	}
	m.view = viewKanban
	m.focusIssue = m.assignment.key
	if m.boardCur >= 0 && m.boardCur < len(m.boards) {
		return m.startBoardLoad(m.boards[m.boardCur])
	}
	return m, nil
}

func (m Model) assignmentView() string {
	status := m.assignment.validation
	if m.assignment.busy {
		status = "Submitting once; awaiting Jira..."
	}
	if m.err != nil {
		status = errStyle.Render(m.err.Error())
	}
	return titleStyle.Render("Assign "+m.assignment.key) + "\n\n" + m.assignment.input.View() + "\n" + status +
		"\n[Enter] Assign  [Esc] Cancel\nEsc leaves this form; an in-flight HTTP mutation cannot be cancelled."
}
