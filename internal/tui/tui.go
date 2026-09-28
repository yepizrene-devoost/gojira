package tui

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/x/ansi"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// ─── Styles ────────────────────────────────────────────────────────────

const colWidth = 24
const colGap = 2

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	subStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Bold(true)
	colHeaderSty = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#4A5568"))
	keyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true)
	metaStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Italic(true)
	selBg        = lipgloss.Color("#3D2B6B")
	detailBox    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#7D56F4")).Padding(1, 2)
)

type helpEntry struct {
	keys   string
	action string
}

var kanbanHelp = []helpEntry{
	{keys: "←/→ or h/l", action: "Move between columns"},
	{keys: "↑/↓ or k/j", action: "Move within a column"},
	{keys: "Enter", action: "Open ticket detail"},
	{keys: "/", action: "Filter loaded tickets; Enter searches Jira"},
	{keys: "s", action: "Search all Jira tickets"},
	{keys: "t", action: "Transition ticket"},
	{keys: "n", action: "Create issue"},
	{keys: "a", action: "Add ticket to active sprint"},
	{keys: "C", action: "Copy column as JSON"},
	{keys: "Esc or q", action: "Back to boards"},
	{keys: "Ctrl+C", action: "Quit"},
}

// ─── Helpers ───────────────────────────────────────────────────────────

func padRight(s string, w int) string {
	n := ansi.StringWidth(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

func truncateRunes(s string, max int) string {
	if max < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func openBrowser(key, domain string) tea.Cmd {
	return func() tea.Msg {
		url := "https://" + domain + "/browse/" + key
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		}
		if cmd != nil {
			if err := cmd.Start(); err != nil {
				return toastMsg{text: "Could not open browser: " + err.Error()}
			}
		}
		return nil
	}
}

func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.WriteAll(text)
		return clipboardMsg{text: text, err: err}
	}
}

func clearToastAfter(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(_ time.Time) tea.Msg {
		return toastMsg{text: ""}
	})
}

// ─── TUI Types ─────────────────────────────────────────────────────────

type view int

const (
	viewBoards view = iota
	viewKanban
	viewDetail
	viewTransition
	viewWorklog
	viewCreate
	viewSearch
	viewDescriptionEdit
)

type column struct {
	name   string
	issues []jira.Issue
}

// Model is the top-level Bubble Tea model for GoJira's TUI.
// Kept as a single model with NO sub-model message routing.
type Model struct {
	client                        *jira.Client
	domain                        string
	view                          view
	boards                        []jira.Board
	boardCur                      int
	columns                       []column
	colCur                        int
	rowCur                        int
	sprintName                    string
	width                         int
	height                        int
	err                           error
	detail                        *jira.Issue
	detailScroll                  int
	detailLoading                 bool
	detailIssueKey                string
	detailBoardRequest            uint64
	detailSearchRequest           uint64
	detailReturn                  view
	nextDetailRequest             uint64
	activeDetailRequest           uint64
	helpOpen                      bool
	helpScroll                    int
	toast                         string
	transitions                   []jira.Transition
	transCur                      int
	transIssue                    string
	transitionLoading             bool
	nextTransitionRequest         uint64
	activeTransitionRequest       uint64
	transitionSubmitting          bool
	nextTransitionSubmitRequest   uint64
	activeTransitionSubmitRequest uint64
	transitionSubmitBoardRequest  uint64
	transitionSubmitBoardID       int
	transitionSubmitIssue         string
	transitionResultErrBoardID    int
	worklogInput                  textinput.Model
	worklogIssue                  string
	nextWorklogRequest            uint64
	activeWorklogRequest          uint64
	descriptionInput              textarea.Model
	descriptionOriginal           string
	descriptionSubmitting         bool
	descriptionApplied            bool
	descriptionErr                error
	nextDescriptionRequest        uint64
	activeDescriptionRequest      uint64

	filterInput  textinput.Model
	filterActive bool

	searchInput         textinput.Model
	searchQuery         string
	searchEditing       bool
	searchResults       []jira.Issue
	searchCur           int
	searchTotal         int
	searchLoading       bool
	searchCompleted     bool
	searchErr           error
	searchReturn        view
	nextSearchRequest   uint64
	activeSearchRequest uint64

	// Active sprint of the current board (0 = none; used by create + "a")
	sprintID int

	// Create-issue form (viewCreate)
	createProject string
	createTypes   []string
	createTypeCur int
	createField   int // 0 = type, 1 = summary, 2 = description
	createSummary textinput.Model
	createDesc    textinput.Model
	createErr     string

	// Issue key to select after the next board reload
	focusIssue string

	// Board requests are identified so late responses cannot replace newer state.
	nextBoardRequest   uint64
	activeBoardRequest uint64
	loadedBoardRequest uint64
	loadingBoardID     int
	loadingBoardName   string
	boardLoading       bool
}

type boardsMsg struct{ boards []jira.Board }
type boardLoadMsg struct {
	requestID  uint64
	boardID    int
	columns    []column
	sprintName string
	sprintID   int
	err        error
}
type detailMsg struct {
	issue         *jira.Issue
	issueKey      string
	boardRequest  uint64
	searchRequest uint64
	detailRequest uint64
	err           error
}
type searchMsg struct {
	requestID uint64
	issues    []jira.Issue
	total     int
	err       error
}
type clipboardMsg struct {
	text string
	err  error
}
type toastMsg struct{ text string }
type transitionsMsg struct {
	requestID    uint64
	transitions  []jira.Transition
	issueKey     string
	boardRequest uint64
	err          error
}
type transitionDoneMsg struct {
	requestID    uint64
	boardRequest uint64
	issueKey     string
	err          error
}
type worklogDoneMsg struct {
	requestID uint64
	issueKey  string
	err       error
}
type descriptionUpdatedMsg struct {
	requestID uint64
	issueKey  string
	issue     *jira.Issue
	applied   bool
	err       error
}
type createTypesMsg struct {
	types []string
	err   error
}
type createdMsg struct {
	key      string
	sprintID int
	err      error
}
type sprintDoneMsg struct {
	issueKey string
	err      error
}
type errMsg struct{ err error }

// New creates a new TUI model ready to be used with bubbletea.NewProgram.
func New(client *jira.Client, domain string) Model {
	return Model{
		client: client,
		domain: domain,
		view:   viewBoards,
	}
}

// ─── Init ──────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return m.loadBoards
}

func (m Model) loadBoards() tea.Msg {
	boards, err := m.client.GetBoards()
	if err != nil {
		return errMsg{err}
	}
	return boardsMsg{boards}
}

func (m Model) startBoardLoad(board jira.Board) (Model, tea.Cmd) {
	preserveTransitionSubmit := m.transitionSubmitting && m.transitionSubmitBoardID == board.ID
	preserveTransitionError := m.transitionResultErrBoardID == board.ID

	m.nextBoardRequest++
	requestID := m.nextBoardRequest
	m.activeBoardRequest = requestID
	m.loadedBoardRequest = 0
	m.loadingBoardID = board.ID
	m.loadingBoardName = board.Name
	m.boardLoading = true
	m.columns = nil
	m.sprintName = ""
	m.sprintID = 0
	if !preserveTransitionError {
		m.err = nil
		m.transitionResultErrBoardID = 0
	}
	if m.view == viewKanban || m.view == viewTransition ||
		((m.view == viewDetail || m.view == viewDescriptionEdit) && m.detailReturn != viewSearch) {
		m.view = viewKanban
		m.detail = nil
		m.detailScroll = 0
		m.detailLoading = false
		m.detailIssueKey = ""
		m.detailBoardRequest = 0
		m.detailSearchRequest = 0
		m.detailReturn = viewKanban
		m.activeDetailRequest = 0
		m.filterInput.SetValue("")
		m.filterActive = false
		m.activeSearchRequest = 0
		m.helpOpen = false
		m.helpScroll = 0
		m.activeWorklogRequest = 0
		m.descriptionSubmitting = false
		m.descriptionApplied = false
		m.activeDescriptionRequest = 0
		m.descriptionErr = nil
	}
	m.transitionLoading = false
	m.activeTransitionRequest = 0
	if !preserveTransitionSubmit {
		m.transitionSubmitting = false
		m.activeTransitionSubmitRequest = 0
		m.transitionSubmitBoardRequest = 0
		m.transitionSubmitBoardID = 0
		m.transitionSubmitIssue = ""
	}
	m.transitions = nil
	m.transIssue = ""
	m.colCur = 0
	m.rowCur = 0

	return m, func() tea.Msg { return m.loadBoardData(board, requestID) }
}

func (m Model) loadBoardData(board jira.Board, requestID uint64) tea.Msg {
	result := boardLoadMsg{requestID: requestID, boardID: board.ID}

	cfg, err := m.client.GetBoardConfig(board.ID)
	if err != nil {
		result.err = err
		return result
	}

	sprints, err := m.client.GetSprints(board.ID)
	if err != nil {
		result.err = err
		return result
	}

	var activeSprint *jira.Sprint
	for i, s := range sprints {
		if s.State == "active" {
			activeSprint = &sprints[i]
			break
		}
	}

	var issues []jira.Issue
	var backlog []jira.Issue
	sprintName := "All issues"
	sprintID := 0
	if activeSprint != nil {
		sprintName = activeSprint.Name
		sprintID = activeSprint.ID
		issues, err = m.client.GetBoardIssues(board.ID, activeSprint.ID)
		if err != nil {
			result.err = err
			return result
		}
		// Backlog = board issues outside the active sprint. Best-effort:
		// a board/issue failure must not break the sprint view.
		if all, berr := m.client.GetBoardIssues(board.ID, 0); berr == nil {
			inSprint := make(map[string]bool, len(issues))
			for _, i := range issues {
				inSprint[i.Key] = true
			}
			for _, i := range all {
				if inSprint[i.Key] {
					continue
				}
				// Done issues sitting in closed sprints are not backlog either.
				if i.Fields.Status != nil && i.Fields.Status.Category != nil &&
					i.Fields.Status.Category.Key == "done" {
					continue
				}
				backlog = append(backlog, i)
			}
		}
	} else {
		issues, err = m.client.GetBoardIssues(board.ID, 0)
		if err != nil {
			result.err = err
			return result
		}
	}

	statusToCol := map[string]string{}
	colMap := map[string]*column{}
	colOrder := []string{}

	if cfg.ColumnConfig != nil {
		for _, cc := range cfg.ColumnConfig.Columns {
			colMap[cc.Name] = &column{name: cc.Name}
			colOrder = append(colOrder, cc.Name)
			for _, s := range cc.Statuses {
				statusToCol[s.ID] = cc.Name
			}
		}
	}

	for _, iss := range issues {
		if iss.Fields.Status != nil {
			if colName, ok := statusToCol[iss.Fields.Status.ID]; ok {
				colMap[colName].issues = append(colMap[colName].issues, iss)
				continue
			}
		}
		if len(colOrder) > 0 {
			colMap[colOrder[0]].issues = append(colMap[colOrder[0]].issues, iss)
		}
	}

	var cols []column
	for _, name := range colOrder {
		if c, ok := colMap[name]; ok {
			cols = append(cols, *c)
		}
	}

	if len(cols) == 0 {
		cols = append(cols, column{name: "To Do", issues: issues})
	}

	// Append the backlog as a trailing column so unsprinted tickets stay
	// reachable: select one and press "a" to move it into the active sprint.
	if len(backlog) > 0 {
		cols = append(cols, column{name: "Backlog", issues: backlog})
	}

	result.columns = cols
	result.sprintName = sprintName
	result.sprintID = sprintID
	return result
}

// ─── Update ────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.view == viewDetail {
			m.detailScroll = min(m.detailScroll, m.detailMaxScroll())
		} else if m.view == viewDescriptionEdit {
			m.sizeDescriptionInput()
		} else if m.helpOpen {
			m.helpScroll = min(m.helpScroll, m.helpMaxScroll())
		}
		return m, nil

	case boardsMsg:
		m.boards = msg.boards
		return m, nil

	case boardLoadMsg:
		if m.view == viewBoards || msg.requestID != m.activeBoardRequest || msg.boardID != m.loadingBoardID {
			return m, nil
		}
		m.activeBoardRequest = 0
		m.boardLoading = false
		if msg.err != nil {
			m.transitionResultErrBoardID = 0
			m.err = msg.err
			return m, nil
		}
		m.loadedBoardRequest = msg.requestID
		m.columns = msg.columns
		m.sprintName = msg.sprintName
		m.sprintID = msg.sprintID
		if m.transitionResultErrBoardID != msg.boardID {
			m.err = nil
		}
		if m.focusIssue != "" {
			if c, r, ok := findIssuePos(msg.columns, m.focusIssue); ok {
				m.colCur, m.rowCur = c, r
			}
			m.focusIssue = ""
		}
		return m, nil

	case detailMsg:
		if m.view != viewDetail || !m.detailLoading ||
			msg.detailRequest == 0 || msg.detailRequest != m.activeDetailRequest ||
			msg.issueKey == "" || msg.issueKey != m.detailIssueKey {
			return m, nil
		}
		if m.detailReturn == viewSearch {
			if msg.searchRequest == 0 || msg.searchRequest != m.detailSearchRequest || msg.searchRequest != m.activeSearchRequest {
				return m, nil
			}
		} else if msg.boardRequest == 0 || msg.boardRequest != m.loadedBoardRequest || msg.boardRequest != m.detailBoardRequest {
			return m, nil
		}
		m.detailLoading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if msg.issue == nil || msg.issue.Key != msg.issueKey {
			return m, nil
		}
		m.err = nil
		m.detail = msg.issue
		m.detailScroll = 0
		return m, nil

	case searchMsg:
		if m.view != viewSearch || msg.requestID == 0 || msg.requestID != m.activeSearchRequest {
			return m, nil
		}
		m.searchLoading = false
		m.searchCompleted = true
		m.searchErr = msg.err
		m.searchResults = msg.issues
		m.searchTotal = msg.total
		m.searchCur = 0
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case clipboardMsg:
		if msg.err != nil {
			m.toast = "Clipboard error: " + msg.err.Error()
			return m, nil
		}
		n := len(msg.text)
		if n > 200 {
			n = 200
		}
		m.toast = fmt.Sprintf("Copied %d chars to clipboard", n)
		return m, clearToastAfter(3 * time.Second)

	case toastMsg:
		m.toast = msg.text
		return m, nil

	case transitionsMsg:
		if m.view != viewKanban || m.boardLoading || !m.transitionLoading ||
			msg.requestID == 0 || msg.requestID != m.activeTransitionRequest ||
			msg.issueKey == "" || msg.issueKey != m.transIssue ||
			msg.boardRequest == 0 || msg.boardRequest != m.loadedBoardRequest {
			return m, nil
		}
		m.transitionLoading = false
		m.activeTransitionRequest = 0
		if msg.err != nil {
			m.transIssue = ""
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.transitions = msg.transitions
		m.transCur = 0
		m.view = viewTransition
		return m, nil

	case transitionDoneMsg:
		if !m.transitionSubmitting ||
			msg.requestID == 0 || msg.requestID != m.activeTransitionSubmitRequest ||
			msg.issueKey == "" || msg.issueKey != m.transitionSubmitIssue ||
			msg.boardRequest == 0 || msg.boardRequest != m.transitionSubmitBoardRequest ||
			m.transitionSubmitBoardID == 0 || m.boardCur < 0 || m.boardCur >= len(m.boards) ||
			m.boards[m.boardCur].ID != m.transitionSubmitBoardID {
			return m, nil
		}
		submitBoardID := m.transitionSubmitBoardID
		m.transitionSubmitting = false
		m.activeTransitionSubmitRequest = 0
		m.transitionSubmitBoardRequest = 0
		m.transitionSubmitBoardID = 0
		m.transitionSubmitIssue = ""
		m.transitions = nil
		m.transIssue = ""
		m.view = viewKanban
		if msg.err != nil {
			m.err = msg.err
			m.transitionResultErrBoardID = submitBoardID
			return m, nil
		}
		m.err = nil
		m.transitionResultErrBoardID = 0
		m.toast = fmt.Sprintf("Moved %s", msg.issueKey)
		m.colCur = 0
		m.rowCur = 0
		loaded, cmd := m.startBoardLoad(m.boards[m.boardCur])
		return loaded, cmd

	case worklogDoneMsg:
		if m.view != viewWorklog || msg.requestID == 0 || msg.requestID != m.activeWorklogRequest ||
			msg.issueKey == "" || msg.issueKey != m.worklogIssue || m.detail == nil || msg.issueKey != m.detail.Key {
			return m, nil
		}
		m.activeWorklogRequest = 0
		m.view = viewDetail
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.toast = "Worklog added"
		return m, clearToastAfter(3 * time.Second)

	case descriptionUpdatedMsg:
		if m.view != viewDescriptionEdit || !m.descriptionSubmitting ||
			msg.requestID == 0 || msg.requestID != m.activeDescriptionRequest ||
			m.detail == nil || msg.issueKey == "" || msg.issueKey != m.detail.Key {
			return m, nil
		}
		m.descriptionSubmitting = false
		m.activeDescriptionRequest = 0
		if msg.err != nil {
			m.descriptionApplied = msg.applied
			m.descriptionErr = msg.err
			return m, nil
		}
		if msg.issue == nil || msg.issue.Key != msg.issueKey {
			m.descriptionApplied = msg.applied
			m.descriptionErr = fmt.Errorf("description was updated, but Jira returned no refreshed ticket")
			return m, nil
		}
		m.descriptionApplied = false
		m.descriptionErr = nil
		m.err = nil
		m.detail = msg.issue
		m.detailScroll = 0
		m.descriptionInput.Blur()
		m.view = viewDetail
		m.toast = "Description updated"
		return m, clearToastAfter(3 * time.Second)

	case createTypesMsg:
		// The form already defaults to "Task"; richer types are a bonus.
		if msg.err == nil && len(msg.types) > 0 {
			m.createTypes = msg.types
			m.createTypeCur = 0
		}
		return m, nil

	case createdMsg:
		if msg.key == "" {
			// Creation failed: stay in the form, keep what was typed.
			m.createErr = msg.err.Error()
			return m, nil
		}
		m.createSummary.Blur()
		m.createDesc.Blur()
		m.view = viewKanban
		m.focusIssue = msg.key
		switch {
		case msg.err != nil:
			m.toast = "Created " + msg.key + " but sprint add failed: " + msg.err.Error()
		case msg.sprintID > 0:
			m.toast = "Created " + msg.key + " → " + m.sprintName
		default:
			m.toast = "Created " + msg.key
		}
		board := m.boards[m.boardCur]
		loaded, loadCmd := m.startBoardLoad(board)
		return loaded, tea.Batch(loadCmd, clearToastAfter(4*time.Second))

	case sprintDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.toast = "Added " + msg.issueKey + " to " + m.sprintName
		m.focusIssue = msg.issueKey
		board := m.boards[m.boardCur]
		loaded, loadCmd := m.startBoardLoad(board)
		return loaded, tea.Batch(loadCmd, clearToastAfter(3*time.Second))

	case tea.KeyPressMsg:
		switch m.view {
		case viewBoards:
			return m.updateBoards(msg)
		case viewKanban:
			return m.updateKanban(msg)
		case viewDetail:
			return m.updateDetail(msg)
		case viewTransition:
			return m.updateTransition(msg)
		case viewWorklog:
			return m.updateWorklog(msg)
		case viewCreate:
			return m.updateCreate(msg)
		case viewSearch:
			return m.updateSearch(msg)
		case viewDescriptionEdit:
			return m.updateDescriptionEdit(msg)
		}

	case tea.PasteMsg:
		switch m.view {
		case viewWorklog:
			var cmd tea.Cmd
			m.worklogInput, cmd = m.worklogInput.Update(msg)
			return m, cmd
		case viewCreate:
			var cmd tea.Cmd
			switch m.createField {
			case 1:
				m.createSummary, cmd = m.createSummary.Update(msg)
			case 2:
				m.createDesc, cmd = m.createDesc.Update(msg)
			}
			return m, cmd
		case viewKanban:
			if m.filterActive {
				var cmd tea.Cmd
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.colCur, m.rowCur = 0, 0
				return m, cmd
			}
		case viewSearch:
			if m.searchEditing {
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				return m, cmd
			}
		case viewDescriptionEdit:
			if !m.descriptionSubmitting && !m.descriptionApplied {
				var cmd tea.Cmd
				m.descriptionInput, cmd = m.descriptionInput.Update(msg)
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m Model) updateBoards(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.boardCur > 0 {
			m.boardCur--
		}
	case "down", "j":
		if m.boardCur < len(m.boards)-1 {
			m.boardCur++
		}
	case "enter":
		if m.boardCur >= 0 && m.boardCur < len(m.boards) {
			board := m.boards[m.boardCur]
			m.view = viewKanban
			m.focusIssue = ""
			loaded, cmd := m.startBoardLoad(board)
			return loaded, cmd
		}
	case "s":
		return m.openSearchInput()
	}
	return m, nil
}

func (m Model) updateKanban(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.helpOpen {
		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "?", "esc", "q":
			m.helpOpen = false
			m.helpScroll = 0
		case "down", "j":
			m.helpScroll = min(m.helpScroll+1, m.helpMaxScroll())
		case "up", "k":
			m.helpScroll = max(m.helpScroll-1, 0)
		case "pgdown":
			m.helpScroll = min(m.helpScroll+m.helpPageSize(), m.helpMaxScroll())
		case "pgup":
			m.helpScroll = max(m.helpScroll-m.helpPageSize(), 0)
		}
		return m, nil
	}
	if m.filterActive {
		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.filterInput.Blur()
			m.filterInput.SetValue("")
			m.filterActive = false
			m.colCur, m.rowCur = 0, 0
			return m, nil
		case "enter":
			query := strings.TrimSpace(m.filterInput.Value())
			if query == "" {
				return m, nil
			}
			m.filterInput.Blur()
			m.filterActive = false
			return m.startSearch(query)
		default:
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.colCur, m.rowCur = 0, 0
			return m, cmd
		}
	}
	if m.transitionLoading {
		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.transitionLoading = false
			m.activeTransitionRequest = 0
			m.transIssue = ""
			return m, nil
		case "q":
			m.transitionLoading = false
			m.activeTransitionRequest = 0
			m.transIssue = ""
		default:
			return m, nil
		}
	}
	if m.boardLoading && key != "ctrl+c" && key != "q" && key != "esc" {
		return m, nil
	}

	cols := m.visibleColumns()
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if strings.TrimSpace(m.filterInput.Value()) != "" {
			m.filterInput.SetValue("")
			m.colCur, m.rowCur = 0, 0
			return m, nil
		}
		fallthrough
	case "q":
		m.view = viewBoards
		m.colCur = 0
		m.rowCur = 0
		m.activeBoardRequest = 0
		m.loadedBoardRequest = 0
		m.boardLoading = false
		m.transitionLoading = false
		m.activeTransitionRequest = 0
		m.transitionSubmitting = false
		m.activeTransitionSubmitRequest = 0
		m.transitionSubmitBoardRequest = 0
		m.transitionSubmitBoardID = 0
		m.transitionSubmitIssue = ""
		m.transitionResultErrBoardID = 0
		m.transitions = nil
		m.transIssue = ""
		return m, nil
	case "?":
		m.helpOpen = true
		m.helpScroll = 0
	case "left", "h":
		if m.colCur > 0 {
			m.colCur--
			m.rowCur = 0
		}
	case "right", "l":
		if m.colCur < len(cols)-1 {
			m.colCur++
			m.rowCur = 0
		}
	case "up", "k":
		if m.rowCur > 0 {
			m.rowCur--
		}
	case "down", "j":
		if m.colCur < len(cols) && m.rowCur < len(cols[m.colCur].issues)-1 {
			m.rowCur++
		}
	case "enter":
		if m.colCur < len(cols) {
			issues := cols[m.colCur].issues
			if m.rowCur >= 0 && m.rowCur < len(issues) {
				key := issues[m.rowCur].Key
				boardRequest := m.loadedBoardRequest
				m.view = viewDetail
				m.detail = nil
				m.detailScroll = 0
				m.detailLoading = true
				m.detailIssueKey = key
				m.detailBoardRequest = boardRequest
				m.detailSearchRequest = 0
				m.detailReturn = viewKanban
				m.nextDetailRequest++
				detailRequest := m.nextDetailRequest
				m.activeDetailRequest = detailRequest
				m.err = nil
				return m, func() tea.Msg {
					iss, err := m.client.GetIssue(key)
					return detailMsg{issue: iss, issueKey: key, boardRequest: boardRequest, detailRequest: detailRequest, err: err}
				}
			}
		}
	case "/":
		input := textinput.New()
		input.Placeholder = "key or summary"
		input.SetWidth(40)
		input.SetValue(m.filterInput.Value())
		input.Focus()
		m.filterInput = input
		m.filterActive = true
		m.colCur, m.rowCur = 0, 0
		return m, textinput.Blink
	case "s":
		return m.openSearchInput()
	case "o":
		if m.colCur < len(cols) {
			issues := cols[m.colCur].issues
			if m.rowCur >= 0 && m.rowCur < len(issues) {
				return m, openBrowser(issues[m.rowCur].Key, m.domain)
			}
		}
	case "C":
		if m.colCur < len(cols) {
			col := cols[m.colCur]
			tickets := jira.IssuesToTicketJSON(col.issues, m.domain)
			b, _ := json.MarshalIndent(tickets, "", "  ")
			return m, copyToClipboard(string(b))
		}
	case "t":
		if m.transitionSubmitting {
			return m, nil
		}
		if m.colCur < len(cols) {
			issues := cols[m.colCur].issues
			if m.rowCur >= 0 && m.rowCur < len(issues) {
				key := issues[m.rowCur].Key
				boardRequest := m.loadedBoardRequest
				m.nextTransitionRequest++
				requestID := m.nextTransitionRequest
				m.activeTransitionRequest = requestID
				m.transitionLoading = true
				m.transIssue = key
				m.transitionResultErrBoardID = 0
				m.err = nil
				client := m.client
				return m, func() tea.Msg {
					tr, err := client.GetTransitions(key)
					return transitionsMsg{requestID: requestID, transitions: tr, issueKey: key, boardRequest: boardRequest, err: err}
				}
			}
		}
	case "n":
		return m.enterCreate()
	case "a":
		if m.colCur >= len(cols) {
			return m, nil
		}
		issues := cols[m.colCur].issues
		if m.rowCur < 0 || m.rowCur >= len(issues) {
			return m, nil
		}
		if m.sprintID == 0 {
			m.toast = "No active sprint on this board"
			return m, clearToastAfter(3 * time.Second)
		}
		key := issues[m.rowCur].Key
		sprintID := m.sprintID
		client := m.client
		return m, func() tea.Msg {
			err := client.AddIssuesToSprint(sprintID, []string{key})
			return sprintDoneMsg{issueKey: key, err: err}
		}
	}
	return m, nil
}

func (m Model) updateDetail(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "down", "j":
		m = m.scrollDetail(1)
	case "up", "k":
		m = m.scrollDetail(-1)
	case "pgdown":
		m = m.scrollDetail(m.detailPageSize())
	case "pgup":
		m = m.scrollDetail(-m.detailPageSize())
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "enter", "q":
		returnView := m.detailReturn
		if returnView != viewSearch {
			returnView = viewKanban
		}
		m.view = returnView
		m.detail = nil
		m.detailScroll = 0
		m.detailLoading = false
		m.detailIssueKey = ""
		m.detailBoardRequest = 0
		m.detailSearchRequest = 0
		m.detailReturn = viewKanban
		m.activeDetailRequest = 0
		m.err = nil
		return m, nil
	case "o":
		if m.detail != nil {
			return m, openBrowser(m.detail.Key, m.domain)
		}
	case "c":
		if m.detail != nil {
			ticket := jira.IssueToTicketJSON(*m.detail, m.domain)
			b, _ := json.MarshalIndent(ticket, "", "  ")
			return m, copyToClipboard(string(b))
		}
	case "w":
		if m.detail != nil {
			ti := textinput.New()
			ti.Placeholder = "e.g. 30m, 2h, 1d"
			ti.Focus()
			ti.CharLimit = 20
			m.worklogInput = ti
			m.worklogIssue = m.detail.Key
			m.activeWorklogRequest = 0
			m.err = nil
			m.view = viewWorklog
			return m, textinput.Blink
		}
	case "e":
		if m.detail != nil {
			input := textarea.New()
			input.Placeholder = "Ticket description (plain text)"
			input.ShowLineNumbers = false
			description := ""
			if m.detail.Fields.Description != nil {
				description = m.detail.Fields.Description.Flatten()
				input.SetValue(description)
			}
			m.descriptionInput = input
			m.descriptionOriginal = m.descriptionInput.Value()
			m.sizeDescriptionInput()
			m.descriptionSubmitting = false
			m.descriptionApplied = false
			m.descriptionErr = nil
			m.activeDescriptionRequest = 0
			m.err = nil
			m.view = viewDescriptionEdit
			return m, m.descriptionInput.Focus()
		}
	}
	return m, nil
}

func (m *Model) sizeDescriptionInput() {
	width := m.modalWidth() - 2
	if width < 1 {
		width = 1
	}
	height := m.modalHeight() - 10
	if height < 3 {
		height = 3
	}
	if height > 12 {
		height = 12
	}
	m.descriptionInput.SetWidth(width)
	m.descriptionInput.SetHeight(height)
}

func (m Model) updateDescriptionEdit(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.descriptionSubmitting {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.descriptionInput.Blur()
		m.descriptionErr = nil
		m.activeDescriptionRequest = 0
		m.view = viewDetail
		if m.descriptionApplied {
			m.descriptionApplied = false
			m.toast = "Description updated; detail was not refreshed"
			return m, clearToastAfter(4 * time.Second)
		}
		return m, nil
	case "ctrl+s":
		if m.detail == nil {
			return m, nil
		}
		if m.descriptionApplied {
			return m.startDescriptionRefresh()
		}
		if m.descriptionInput.Value() == m.descriptionOriginal {
			m.descriptionInput.Blur()
			m.descriptionErr = nil
			m.view = viewDetail
			return m, nil
		}
		return m.startDescriptionUpdate()
	default:
		if m.descriptionApplied {
			return m, nil
		}
		var cmd tea.Cmd
		m.descriptionInput, cmd = m.descriptionInput.Update(msg)
		return m, cmd
	}
}

func (m Model) startDescriptionUpdate() (tea.Model, tea.Cmd) {
	issueKey := m.detail.Key
	text := m.descriptionInput.Value()
	var description interface{}
	if text != "" {
		description = jira.TextToADF(text)
	}
	m.nextDescriptionRequest++
	requestID := m.nextDescriptionRequest
	m.activeDescriptionRequest = requestID
	m.descriptionSubmitting = true
	m.descriptionErr = nil
	client := m.client
	return m, func() tea.Msg {
		if err := client.UpdateIssue(issueKey, map[string]interface{}{"description": description}); err != nil {
			return descriptionUpdatedMsg{requestID: requestID, issueKey: issueKey, err: err}
		}
		issue, err := client.GetIssueFull(issueKey)
		if err != nil {
			err = fmt.Errorf("description was updated, but detail refresh failed: %w", err)
		}
		return descriptionUpdatedMsg{requestID: requestID, issueKey: issueKey, issue: issue, applied: true, err: err}
	}
}

func (m Model) startDescriptionRefresh() (tea.Model, tea.Cmd) {
	issueKey := m.detail.Key
	m.nextDescriptionRequest++
	requestID := m.nextDescriptionRequest
	m.activeDescriptionRequest = requestID
	m.descriptionSubmitting = true
	m.descriptionErr = nil
	client := m.client
	return m, func() tea.Msg {
		issue, err := client.GetIssueFull(issueKey)
		if err != nil {
			err = fmt.Errorf("description was updated, but detail refresh failed: %w", err)
		}
		return descriptionUpdatedMsg{requestID: requestID, issueKey: issueKey, issue: issue, applied: true, err: err}
	}
}

func (m Model) scrollDetail(steps int) Model {
	if m.detailLoading || m.detail == nil {
		return m
	}
	m.detailScroll = max(0, min(m.detailScroll+steps, m.detailMaxScroll()))
	return m
}

func (m Model) updateTransition(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keyPress := msg.String()
	if m.transitionSubmitting {
		switch keyPress {
		case "ctrl+c":
			return m, tea.Quit
		case "esc", "q":
			m.transitionSubmitting = false
			m.activeTransitionSubmitRequest = 0
			m.transitionSubmitBoardRequest = 0
			m.transitionSubmitBoardID = 0
			m.transitionSubmitIssue = ""
			m.transitions = nil
			m.transIssue = ""
			m.view = viewKanban
		}
		return m, nil
	}

	switch keyPress {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.transitions = nil
		m.transIssue = ""
		m.view = viewKanban
		return m, nil
	case "up", "k":
		if m.transCur > 0 {
			m.transCur--
		}
	case "down", "j":
		if m.transCur < len(m.transitions)-1 {
			m.transCur++
		}
	case "enter":
		if m.transCur >= 0 && m.transCur < len(m.transitions) {
			tr := m.transitions[m.transCur]
			issueKey := m.transIssue
			boardRequest := m.loadedBoardRequest
			boardID := 0
			if m.boardCur >= 0 && m.boardCur < len(m.boards) {
				boardID = m.boards[m.boardCur].ID
			}
			if boardRequest == 0 || boardID == 0 {
				return m, nil
			}
			m.nextTransitionSubmitRequest++
			requestID := m.nextTransitionSubmitRequest
			m.activeTransitionSubmitRequest = requestID
			m.transitionSubmitBoardRequest = boardRequest
			m.transitionSubmitBoardID = boardID
			m.transitionSubmitIssue = issueKey
			m.transitionSubmitting = true
			m.transitionResultErrBoardID = 0
			m.err = nil
			client := m.client
			return m, func() tea.Msg {
				err := client.TransitionIssue(issueKey, tr.ID)
				return transitionDoneMsg{requestID: requestID, boardRequest: boardRequest, issueKey: issueKey, err: err}
			}
		}
	}
	return m, nil
}

func (m Model) updateWorklog(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.activeWorklogRequest = 0
		m.err = nil
		m.view = viewDetail
		return m, nil
	case "enter":
		timeSpent := m.worklogInput.Value()
		if timeSpent == "" {
			return m, nil
		}
		key := m.worklogIssue
		m.nextWorklogRequest++
		requestID := m.nextWorklogRequest
		m.activeWorklogRequest = requestID
		return m, func() tea.Msg {
			err := m.client.AddWorklog(key, timeSpent, "")
			return worklogDoneMsg{requestID: requestID, issueKey: key, err: err}
		}
	default:
		var cmd tea.Cmd
		m.worklogInput, cmd = m.worklogInput.Update(msg)
		return m, cmd
	}
}

// ─── Search ────────────────────────────────────────────────────────────

func (m Model) visibleColumns() []column {
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if query == "" {
		return m.columns
	}
	cols := make([]column, len(m.columns))
	for i, col := range m.columns {
		cols[i].name = col.name
		for _, issue := range col.issues {
			if strings.Contains(strings.ToLower(issue.Key), query) || strings.Contains(strings.ToLower(issue.Fields.Summary), query) {
				cols[i].issues = append(cols[i].issues, issue)
			}
		}
	}
	return cols
}

func (m Model) openSearchInput() (tea.Model, tea.Cmd) {
	returnView := m.view
	if returnView != viewBoards {
		returnView = viewKanban
	}
	input := textinput.New()
	input.Placeholder = "Jira text search"
	input.SetWidth(50)
	input.Focus()
	m.searchInput = input
	m.searchQuery = ""
	m.searchEditing = true
	m.searchResults = nil
	m.searchCur = 0
	m.searchTotal = 0
	m.searchLoading = false
	m.searchCompleted = false
	m.searchErr = nil
	m.searchReturn = returnView
	m.activeSearchRequest = 0
	m.view = viewSearch
	return m, textinput.Blink
}

func (m Model) startSearch(query string) (tea.Model, tea.Cmd) {
	if m.view != viewSearch {
		m.searchReturn = viewKanban
	}
	if m.searchInput.Value() != query {
		input := textinput.New()
		input.SetWidth(50)
		input.SetValue(query)
		m.searchInput = input
	}
	m.searchInput.Blur()
	m.searchQuery = query
	m.searchEditing = false
	m.searchResults = nil
	m.searchCur = 0
	m.searchTotal = 0
	m.searchLoading = true
	m.searchCompleted = false
	m.searchErr = nil
	m.nextSearchRequest++
	requestID := m.nextSearchRequest
	m.activeSearchRequest = requestID
	m.view = viewSearch
	client := m.client
	return m, func() tea.Msg {
		issues, total, err := client.SearchText(query, 50)
		return searchMsg{requestID: requestID, issues: issues, total: total, err: err}
	}
}

func (m Model) exitSearch() Model {
	m.searchInput.Blur()
	m.searchEditing = false
	m.searchLoading = false
	m.activeSearchRequest = 0
	if m.searchReturn == viewBoards {
		m.view = viewBoards
	} else {
		m.view = viewKanban
	}
	return m
}

func (m Model) updateSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.searchEditing {
		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.searchInput.Blur()
			m.searchEditing = false
			if m.searchCompleted {
				m.searchInput.SetValue(m.searchQuery)
				return m, nil
			}
			m = m.exitSearch()
			return m, nil
		case "enter":
			query := strings.TrimSpace(m.searchInput.Value())
			if query == "" {
				return m, nil
			}
			return m.startSearch(query)
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}
	}

	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m = m.exitSearch()
		return m, nil
	case "s":
		m.searchInput.Focus()
		m.searchEditing = true
		return m, textinput.Blink
	case "up", "k":
		if m.searchCur > 0 {
			m.searchCur--
		}
	case "down", "j":
		if m.searchCur < len(m.searchResults)-1 {
			m.searchCur++
		}
	case "enter":
		if m.searchLoading || m.searchCur < 0 || m.searchCur >= len(m.searchResults) {
			return m, nil
		}
		key := m.searchResults[m.searchCur].Key
		m.view = viewDetail
		m.detail = nil
		m.detailScroll = 0
		m.detailLoading = true
		m.detailIssueKey = key
		m.detailBoardRequest = 0
		m.detailSearchRequest = m.activeSearchRequest
		m.detailReturn = viewSearch
		m.nextDetailRequest++
		detailRequest := m.nextDetailRequest
		m.activeDetailRequest = detailRequest
		m.err = nil
		searchRequest := m.activeSearchRequest
		client := m.client
		return m, func() tea.Msg {
			issue, err := client.GetIssueFull(key)
			return detailMsg{issue: issue, issueKey: key, searchRequest: searchRequest, detailRequest: detailRequest, err: err}
		}
	}
	return m, nil
}

// ─── Create issue ──────────────────────────────────────────────────────

// enterCreate opens the new-issue form for the current board's project and
// fetches the creatable issue types in the background.
func (m Model) enterCreate() (tea.Model, tea.Cmd) {
	if m.boardCur >= len(m.boards) {
		return m, nil
	}
	board := m.boards[m.boardCur]
	project := ""
	if board.Location != nil {
		project = board.Location.ProjectKey
	}
	if project == "" {
		m.toast = "Board has no project location — cannot create here"
		return m, clearToastAfter(3 * time.Second)
	}

	sum := textinput.New()
	sum.Placeholder = "required"
	sum.CharLimit = 255
	sum.SetWidth(40)
	desc := textinput.New()
	desc.Placeholder = "optional, single line"
	desc.CharLimit = 2000
	desc.SetWidth(40)

	m.createProject = project
	m.createTypes = []string{"Task"} // replaced by createmeta when it lands
	m.createTypeCur = 0
	m.createField = 0
	m.createSummary = sum
	m.createDesc = desc
	m.createErr = ""
	m.view = viewCreate

	client := m.client
	return m, func() tea.Msg {
		names, err := client.GetIssueTypes(project)
		return createTypesMsg{types: names, err: err}
	}
}

func (m Model) cycleCreateField(step int) (tea.Model, tea.Cmd) {
	m.createSummary.Blur()
	m.createDesc.Blur()
	m.createField = (m.createField + step) % 3
	var cmd tea.Cmd
	switch m.createField {
	case 1:
		m.createSummary.Focus()
		cmd = textinput.Blink
	case 2:
		m.createDesc.Focus()
		cmd = textinput.Blink
	}
	return m, cmd
}

func (m Model) updateCreate(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.createSummary.Blur()
		m.createDesc.Blur()
		m.view = viewKanban
		return m, nil
	case "tab":
		return m.cycleCreateField(1)
	case "shift+tab":
		return m.cycleCreateField(2)
	case "enter":
		return m.submitCreate()
	}

	switch m.createField {
	case 0:
		switch msg.String() {
		case "left", "up", "h", "k":
			if m.createTypeCur > 0 {
				m.createTypeCur--
			}
		case "right", "down", "l", "j":
			if m.createTypeCur < len(m.createTypes)-1 {
				m.createTypeCur++
			}
		}
	case 1:
		var cmd tea.Cmd
		m.createSummary, cmd = m.createSummary.Update(msg)
		return m, cmd
	case 2:
		var cmd tea.Cmd
		m.createDesc, cmd = m.createDesc.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) submitCreate() (tea.Model, tea.Cmd) {
	summary := strings.TrimSpace(m.createSummary.Value())
	if summary == "" {
		m.createErr = "Summary is required"
		return m, nil
	}
	issueType := "Task"
	if m.createTypeCur < len(m.createTypes) {
		issueType = m.createTypes[m.createTypeCur]
	}
	var doc *jira.ADFDoc
	if d := strings.TrimSpace(m.createDesc.Value()); d != "" {
		a := jira.TextToADF(d)
		doc = &a
	}
	client := m.client
	project := m.createProject
	sprintID := m.sprintID
	m.createErr = ""
	return m, func() tea.Msg {
		key, err := client.CreateIssue(project, issueType, summary, doc)
		if err != nil {
			return createdMsg{err: err}
		}
		if sprintID > 0 {
			if serr := client.AddIssuesToSprint(sprintID, []string{key}); serr != nil {
				return createdMsg{key: key, sprintID: sprintID, err: serr}
			}
		}
		return createdMsg{key: key, sprintID: sprintID}
	}
}

// findIssuePos locates an issue inside the board columns so the cursor can
// be placed on it after a reload.
func findIssuePos(cols []column, key string) (int, int, bool) {
	for c, col := range cols {
		for r, iss := range col.issues {
			if iss.Key == key {
				return c, r, true
			}
		}
	}
	return 0, 0, false
}

// ─── View ──────────────────────────────────────────────────────────────

func (m Model) View() tea.View {
	var b strings.Builder

	switch m.view {
	case viewBoards:
		b.WriteString(titleStyle.Render("🚀 GoJira TUI"))
		b.WriteString("\n\n")
		if m.err != nil {
			b.WriteString(errStyle.Render("Error: " + m.err.Error()))
			b.WriteString("\n\n")
		}
		if len(m.boards) == 0 {
			b.WriteString(subStyle.Render("Loading boards..."))
			return altScreenView(b.String())
		}
		for i, board := range m.boards {
			icon := "📋"
			switch board.Type {
			case "scrum":
				icon = "🏉"
			case "kanban":
				icon = "📊"
			}
			// Show project name if available (e.g. "SCRUM board · Access Road Assistance")
			label := board.Name
			if board.Location != nil && board.Location.ProjectName != "" {
				label = board.Name + " · " + board.Location.ProjectName
			}
			line := fmt.Sprintf("%s %s [%s]", icon, label, board.Type)
			if i == m.boardCur {
				b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true).Render("→ " + line))
			} else {
				b.WriteString("  " + line)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(subStyle.Render("[↑↓] Navigate  [Enter] Select  [s] Search  [q] Quit"))

	case viewKanban:
		board := m.renderKanban()
		if m.helpOpen {
			b.WriteString(m.composeOverlay(board, m.renderHelp()))
		} else {
			b.WriteString(board)
		}

	case viewDetail:
		background := m.renderKanban()
		if m.detailReturn == viewSearch {
			background = m.renderSearch()
		}
		b.WriteString(m.composeOverlay(background, m.renderDetail()))

	case viewTransition:
		b.WriteString(titleStyle.Render("🔄 Transition: " + m.transIssue))
		b.WriteString("\n\n")
		if m.transitionSubmitting && m.transCur >= 0 && m.transCur < len(m.transitions) {
			b.WriteString(subStyle.Render(fmt.Sprintf("Moving %s to %s...", m.transIssue, m.transitions[m.transCur].Name)))
			b.WriteString("\n")
			b.WriteString(subStyle.Render("[esc] Cancel  [ctrl+c] Quit"))
		} else {
			for i, tr := range m.transitions {
				if i == m.transCur {
					b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true).Render("→ " + tr.Name))
				} else {
					b.WriteString("  " + tr.Name)
				}
				b.WriteString("\n")
			}
			b.WriteString("\n")
			b.WriteString(subStyle.Render("[↑↓] Navigate  [Enter] Confirm  [esc] Cancel"))
		}

	case viewWorklog:
		b.WriteString(titleStyle.Render("⏱ Worklog: " + m.worklogIssue))
		b.WriteString("\n\n")
		b.WriteString(subStyle.Render("Time spent (e.g. 30m, 2h, 1d):"))
		b.WriteString("\n")
		b.WriteString(m.worklogInput.View())
		b.WriteString("\n\n")
		b.WriteString(subStyle.Render("[Enter] Confirm  [esc] Cancel"))

	case viewCreate:
		b.WriteString(titleStyle.Render("✚ New Issue · " + m.createProject))
		b.WriteString("\n\n")

		cur := "Task"
		if m.createTypeCur < len(m.createTypes) {
			cur = m.createTypes[m.createTypeCur]
		}
		fields := []string{"Type", "Summary", "Description"}
		for i, label := range fields {
			marker := "  "
			if i == m.createField {
				marker = "→ "
			}
			var line string
			switch i {
			case 0:
				hint := ""
				if len(m.createTypes) > 1 {
					hint = fmt.Sprintf("  (%d/%d)", m.createTypeCur+1, len(m.createTypes))
				}
				line = fmt.Sprintf("◀ %s ▶%s", cur, hint)
			case 1:
				line = m.createSummary.View()
			case 2:
				line = m.createDesc.View()
			}
			b.WriteString(lipgloss.NewStyle().Bold(i == m.createField).Render(marker+padRight(label+":", 14)) + line + "\n")
		}
		b.WriteString("\n")

		if m.sprintID > 0 {
			b.WriteString(subStyle.Render("Enter creates it directly in sprint: " + m.sprintName))
		} else {
			b.WriteString(subStyle.Render("No active sprint — the ticket will land in the backlog"))
		}
		b.WriteString("\n")
		if m.createErr != "" {
			b.WriteString(errStyle.Render("✗ " + m.createErr))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(subStyle.Render("[tab] Field  [←→] Type  [Enter] Create  [esc] Cancel"))

	case viewSearch:
		b.WriteString(m.renderSearch())

	case viewDescriptionEdit:
		background := m.renderKanban()
		if m.detailReturn == viewSearch {
			background = m.renderSearch()
		}
		b.WriteString(m.composeOverlay(background, m.renderDescriptionEdit()))
	}

	content := b.String()
	if m.view == viewTransition && m.width > 0 && m.contentHeight() > 0 {
		content = fitToBounds(content, m.width, m.contentHeight())
	}
	return altScreenView(m.renderToast(content))
}

func (m Model) renderToast(content string) string {
	if m.toast == "" {
		return content
	}

	toast := lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Bold(true).Render("✓ " + m.toast)
	if m.width <= 0 || m.height <= 0 {
		return content + "\n" + toast
	}

	toast = fitToBounds(toast, m.width, 1)
	if m.height == 1 || content == "" {
		return toast
	}
	return fitToBounds(content, m.width, m.contentHeight()) + "\n" + toast
}

func (m Model) contentHeight() int {
	if m.height <= 0 {
		return m.height
	}
	if m.toast != "" {
		return max(m.height-1, 0)
	}
	return m.height
}

func (m Model) renderSearch() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("🔎 Jira Search"))
	b.WriteString("\n\n")
	b.WriteString("Query: ")
	b.WriteString(m.searchInput.View())
	b.WriteString("\n\n")

	switch {
	case m.searchEditing:
		b.WriteString(subStyle.Render("Type a text query, then press Enter."))
	case m.searchLoading:
		b.WriteString(subStyle.Render("Searching Jira..."))
	case m.searchErr != nil:
		b.WriteString(errStyle.Render("Error: " + m.searchErr.Error()))
	case m.searchCompleted && len(m.searchResults) == 0:
		b.WriteString(subStyle.Render("No tickets found."))
	default:
		if m.searchCompleted {
			_, _ = fmt.Fprintf(&b, "%d shown · %d total\n\n", len(m.searchResults), m.searchTotal)
		}
		visible := len(m.searchResults)
		if m.contentHeight() > 0 {
			visible = min(visible, max(m.contentHeight()-8, 1))
		}
		start := 0
		if m.searchCur >= visible {
			start = m.searchCur - visible + 1
		}
		for i := start; i < min(start+visible, len(m.searchResults)); i++ {
			issue := m.searchResults[i]
			status := ""
			if issue.Fields.Status != nil {
				status = " · " + issue.Fields.Status.Name
			}
			line := fmt.Sprintf("%s  %s%s", issue.Key, strings.ReplaceAll(issue.Fields.Summary, "\n", " "), status)
			if m.width > 0 {
				line = ansi.Truncate(line, max(m.width-4, 1), "…")
			}
			if i == m.searchCur {
				b.WriteString(lipgloss.NewStyle().Background(selBg).Render("→ " + line))
			} else {
				b.WriteString("  " + line)
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	if m.searchEditing {
		b.WriteString(subStyle.Render("[Enter] Search  [esc] Cancel  [ctrl+c] Quit"))
	} else {
		back := "Board"
		if m.searchReturn == viewBoards {
			back = "Boards"
		}
		b.WriteString(subStyle.Render("[↑↓] Navigate  [Enter] Detail  [s] New search  [esc] " + back))
	}
	content := b.String()
	if m.width > 0 && m.contentHeight() > 0 {
		return fitToBounds(content, m.width, m.contentHeight())
	}
	return content
}

func (m Model) renderKanban() string {
	var b strings.Builder
	cols := m.visibleColumns()
	boardName := m.loadingBoardName
	if !m.boardLoading && m.boardCur >= 0 && m.boardCur < len(m.boards) {
		boardName = m.boards[m.boardCur].Name
	}
	b.WriteString(titleStyle.Render("📋 Kanban"))
	b.WriteString("\n")

	if m.boardLoading {
		b.WriteString(subStyle.Render(m.kanbanStatus(boardName, "Loading", 0, 0, 0, 0)))
		b.WriteString("\n\n")
		b.WriteString(subStyle.Render("Loading " + boardName + "..."))
		return m.boundKanban(b.String())
	}

	cpp := max(m.colsPerPage(), 1)
	pageStart := visiblePageStart(m.colCur, cpp, len(cols))
	pageEnd := min(pageStart+cpp, len(cols))
	totalPages := max((len(cols)+cpp-1)/cpp, 1)
	curPage := min(m.colCur/cpp+1, totalPages)
	status := m.kanbanStatus(boardName, m.sprintName, m.colCur+1, len(cols), curPage, totalPages)
	b.WriteString(subStyle.Render(status))
	b.WriteString("\n")
	if m.filterActive || strings.TrimSpace(m.filterInput.Value()) != "" {
		b.WriteString("Filter: ")
		b.WriteString(m.filterInput.View())
		b.WriteString("\n")
	}
	if m.transitionLoading {
		b.WriteString(subStyle.Render("Loading transitions for " + m.transIssue + "...  [esc] Cancel"))
		b.WriteString("\n")
	}

	if m.err != nil && m.view == viewKanban {
		b.WriteString("\n")
		b.WriteString(errStyle.Render("Error: " + m.err.Error()))
		return m.boundKanban(b.String())
	}
	if len(cols) == 0 {
		b.WriteString(subStyle.Render("No board columns available."))
		return m.boundKanban(b.String())
	}

	footer := m.kanbanFooter(curPage, totalPages, len(cols))
	footerHeight := lipgloss.Height(footer)
	height := m.contentHeight()
	headerRows := 1 + lipgloss.Height(status)
	if m.filterActive || strings.TrimSpace(m.filterInput.Value()) != "" {
		headerRows++
	}
	if m.transitionLoading {
		headerRows++
	}
	colH := max(height-headerRows-footerHeight-1, 1)
	slots := max((colH-1)/4, 0)
	cw := colWidth + colGap
	colStrs := make([]string, 0, pageEnd-pageStart)
	for i := pageStart; i < pageEnd; i++ {
		colStrs = append(colStrs, m.renderColumn(cols[i], i == m.colCur, cw, colH, slots))
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, colStrs...))
	b.WriteString("\n\n")
	b.WriteString(subStyle.Render(footer))

	return m.boundKanban(b.String())
}

func (m Model) boundKanban(content string) string {
	if m.width > 0 && m.contentHeight() > 0 {
		return fitToBounds(content, m.width, m.contentHeight())
	}
	return content
}

func (m Model) kanbanStatus(boardName, sprintName string, col, totalCols, page, totalPages int) string {
	if sprintName == "" {
		sprintName = "All issues"
	}
	context := fmt.Sprintf("Board: %s  Sprint: %s", boardName, sprintName)
	if totalCols == 0 {
		return context
	}
	position := fmt.Sprintf("Column %d/%d  Page %d/%d", col, totalCols, page, totalPages)
	full := context + "  " + position
	if m.width <= 0 || (m.width >= 100 && ansi.StringWidth(full) <= m.width) {
		return full
	}

	compactPosition := fmt.Sprintf("Col %d/%d  Page %d/%d", col, totalCols, page, totalPages)
	boardLine := "Board: " + boardName
	sprintPrefix := "Sprint: "
	suffix := "  " + compactPosition
	if m.width > 0 {
		boardLine = ansi.Truncate(boardLine, m.width, "…")
		sprintWidth := max(m.width-ansi.StringWidth(sprintPrefix)-ansi.StringWidth(suffix), 1)
		sprintName = ansi.Truncate(sprintName, sprintWidth, "…")
		compactPosition = ansi.Truncate(compactPosition, m.width, "")
	}
	sprintLine := sprintPrefix + sprintName + "  " + compactPosition
	if m.width > 0 {
		sprintLine = ansi.Truncate(sprintLine, m.width, "")
	}
	return boardLine + "\n" + sprintLine
}

func (m Model) kanbanFooter(curPage, totalPages, totalCols int) string {
	full := fmt.Sprintf(
		"[←→] Cols  [↑↓] Issues  [Enter] Detail  [/] Filter  [s] Search  [?] Help  [t] Transition  [n] New  [a] To Sprint  [C] Copy Col  [esc] Back    Col %d/%d  Page %d/%d",
		m.colCur+1, totalCols, curPage, totalPages)
	if m.width <= 0 || ansi.StringWidth(full) <= m.width {
		return full
	}

	const compact = "[?] Help  [esc] Back"
	if ansi.StringWidth(compact) <= m.width {
		return compact
	}
	return "[?] Help\n[esc] Back"
}

func (m Model) helpContent() (string, string) {
	var b strings.Builder
	b.WriteString(keyStyle.Render("? Kanban Help"))
	b.WriteString("\n\n")
	for _, entry := range kanbanHelp {
		_, _ = fmt.Fprintf(&b, "%-14s %s\n", entry.keys, entry.action)
	}
	return b.String(), "[↑↓] Scroll  [? / esc / q] Close"
}

func (m Model) helpMaxScroll() int {
	content, actions := m.helpContent()
	_, _, _, limit := m.detailViewport(content, actions)
	return limit
}

func (m Model) helpPageSize() int {
	content, actions := m.helpContent()
	_, _, visible, _ := m.detailViewport(content, actions)
	return max(visible, 1)
}

func (m Model) renderHelp() string {
	content, actions := m.helpContent()
	return m.renderModal(content, actions, m.helpScroll)
}

func (m Model) modalWidth() int {
	if m.width <= 0 {
		return 94
	}
	return max(min(m.width-8, 94), 1)
}

func (m Model) modalHeight() int {
	if m.contentHeight() <= 0 {
		return 35
	}
	return min(35, max(m.contentHeight()-2, 1))
}

func (m Model) modalStyle() lipgloss.Style {
	return detailBox.Width(m.modalWidth() + 6).Height(m.modalHeight())
}

func (m Model) detailViewport(content, actions string) ([]string, string, int, int) {
	wrapped := strings.Split(ansi.Wrap(strings.TrimRight(content, "\n"), m.modalWidth(), ""), "\n")
	if strings.Contains(actions, "Scroll") && ansi.StringWidth(actions) > m.modalWidth() && (m.modalWidth() < 40 || m.modalHeight() < 10) {
		actions = "[↑↓] Scroll  [esc] Close"
	}
	footer := subStyle.Render(ansi.Wrap(actions, m.modalWidth(), ""))
	// Border and padding use four rows. Keep the title and actions visible.
	visible := min(len(wrapped)-1, max(m.modalHeight()-5-lipgloss.Height(footer), 0))
	return wrapped, footer, visible, max(len(wrapped)-1-visible, 0)
}

func (m Model) detailMaxScroll() int {
	content, actions := m.detailContent()
	_, _, _, limit := m.detailViewport(content, actions)
	return limit
}

func (m Model) detailPageSize() int {
	content, actions := m.detailContent()
	_, _, visible, _ := m.detailViewport(content, actions)
	return max(visible, 1)
}

func (m Model) renderModal(content, actions string, offset int) string {
	wrapped, footer, visible, limit := m.detailViewport(content, actions)
	start := min(offset, limit)
	body := append([]string{wrapped[0]}, wrapped[1+start:1+start+visible]...)
	for len(body)+lipgloss.Height(footer) < m.modalHeight()-4 {
		body = append(body, "")
	}
	body = append(body, footer)
	return m.modalStyle().Render(strings.Join(body, "\n"))
}

func (m Model) renderDetailModal(content, actions string) string {
	return m.renderModal(content, actions, m.detailScroll)
}

func (m Model) composeOverlay(background, overlay string) string {
	width, height := m.width, m.contentHeight()
	if width <= 0 {
		width = max(lipgloss.Width(background), lipgloss.Width(overlay))
	}
	if height <= 0 && m.height <= 0 {
		height = max(lipgloss.Height(background), lipgloss.Height(overlay))
	}
	width = max(width, 1)
	height = max(height, 1)

	if width < 50 || height < 16 {
		return fitToBounds(overlay, width, height)
	}
	background = fitToBounds(background, width, height)
	overlay = fitToBounds(overlay, width-2, height-2)
	x := max((width-lipgloss.Width(overlay))/2, 0)
	y := max((height-lipgloss.Height(overlay))/2, 0)
	compositor := lipgloss.NewCompositor(
		lipgloss.NewLayer(background).Z(0),
		lipgloss.NewLayer(overlay).X(x).Y(y).Z(1),
	)
	return lipgloss.NewCanvas(width, height).Compose(compositor).Render()
}

func fitToBounds(content string, width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "")
	}
	return strings.Join(lines, "\n")
}

func altScreenView(content string) tea.View {
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) colsPerPage() int {
	cw := colWidth + colGap
	if cw < 1 {
		return 1
	}
	return m.width / cw
}

func visiblePageStart(colCur, colsPerPage, total int) int {
	if colsPerPage >= total {
		return 0
	}
	start := 0
	if colCur >= start+colsPerPage {
		start = colCur - colsPerPage + 1
	}
	if colCur < start {
		start = colCur
	}
	return start
}

func (m Model) renderColumn(col column, isActive bool, cw int, colH int, slots int) string {
	lines := make([]string, 0, colH)

	hdr := fmt.Sprintf(" %s (%d) ", truncateRunes(col.name, cw-6), len(col.issues))
	hdr = padRight(hdr, cw)
	hdr = colHeaderSty.Render(hdr)
	lines = append(lines, hdr)

	offset := 0
	if isActive && slots > 0 && len(col.issues) > slots {
		offset = m.rowCur - slots + 1
		if offset < 0 {
			offset = 0
		}
		maxOff := len(col.issues) - slots
		if maxOff < 0 {
			maxOff = 0
		}
		if offset > maxOff {
			offset = maxOff
		}
	}

	for i := 0; i < slots; i++ {
		idx := offset + i
		if idx < len(col.issues) {
			iss := col.issues[idx]
			sel := isActive && idx == m.rowCur
			lines = append(lines, m.renderCardLines(iss, sel, cw)...)
		} else {
			lines = append(lines, padRight("", cw))
			lines = append(lines, padRight("", cw))
			lines = append(lines, padRight("", cw))
		}
		lines = append(lines, padRight("", cw))
	}

	for len(lines) < colH {
		lines = append(lines, padRight("", cw))
	}
	if len(lines) > colH {
		lines = lines[:colH]
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderCardLines(iss jira.Issue, selected bool, cw int) []string {
	key := truncateRunes(iss.Key, cw-2)
	keyLine := keyStyle.Render(" " + key)
	if selected {
		keyLine = lipgloss.NewStyle().Background(selBg).Render(padRight(keyLine, cw))
	} else {
		keyLine = padRight(keyLine, cw)
	}

	summary := truncateRunes(iss.Fields.Summary, cw-4)
	summaryLine := " " + summary
	if selected {
		summaryLine = padRight(summaryLine, cw)
		summaryLine = lipgloss.NewStyle().Background(selBg).Render(summaryLine)
	} else {
		summaryLine = padRight(summaryLine, cw)
	}

	pri := ""
	if iss.Fields.Priority != nil {
		pri = iss.Fields.Priority.Name
	}
	who := "Unassigned"
	if iss.Fields.Assignee != nil {
		who = iss.Fields.Assignee.DisplayName
	}
	meta := truncateRunes(fmt.Sprintf(" %s • %s", pri, who), cw-2)
	metaLine := metaStyle.Render(meta)
	if selected {
		metaLine = lipgloss.NewStyle().Background(selBg).Render(padRight(metaLine, cw))
	} else {
		metaLine = padRight(metaLine, cw)
	}

	return []string{keyLine, summaryLine, metaLine}
}

func (m Model) renderDetail() string {
	content, actions := m.detailContent()
	return m.renderDetailModal(content, actions)
}

func (m Model) renderDescriptionEdit() string {
	key := m.detailIssueKey
	if m.detail != nil {
		key = m.detail.Key
	}
	var b strings.Builder
	b.WriteString(keyStyle.Render("✎ Edit description · " + key))
	b.WriteString("\n\n")
	b.WriteString(m.descriptionInput.View())
	if m.descriptionApplied {
		b.WriteString("\n")
		b.WriteString(subStyle.Render("The description was updated in Jira; only the detail refresh failed."))
	}
	if m.descriptionErr != nil {
		b.WriteString("\n")
		b.WriteString(errStyle.Render("Error: " + m.descriptionErr.Error()))
	}
	b.WriteString("\n\n")
	if m.descriptionSubmitting {
		if m.descriptionApplied {
			b.WriteString(subStyle.Render("Refreshing ticket detail...  [ctrl+c] Quit"))
		} else {
			b.WriteString(subStyle.Render("Updating description...  [ctrl+c] Quit"))
		}
	} else if m.descriptionApplied {
		b.WriteString(subStyle.Render("[ctrl+s] Retry refresh only  [esc] Close (update remains applied)"))
	} else {
		b.WriteString(subStyle.Render("[ctrl+s] Confirm  [esc] Cancel"))
	}
	return m.modalStyle().Render(b.String())
}

func (m Model) detailContent() (string, string) {
	var b strings.Builder
	if m.detailLoading {
		b.WriteString(keyStyle.Render("📋 " + m.detailIssueKey))
		b.WriteString("\n\n")
		b.WriteString(subStyle.Render("Loading ticket detail..."))
		b.WriteString("\n\n")
		return b.String(), "[esc] Close  [ctrl+c] Quit"
	}
	if m.err != nil {
		b.WriteString(keyStyle.Render("📋 " + m.detailIssueKey))
		b.WriteString("\n\n")
		b.WriteString(errStyle.Render("Error: " + m.err.Error()))
		b.WriteString("\n\n")
		return b.String(), "[esc] Close"
	}
	if m.detail == nil {
		return "Ticket detail unavailable", "[esc] Close"
	}

	iss := m.detail
	key := keyStyle.Render(fmt.Sprintf("📋 %s", iss.Key))
	summary := strings.TrimSpace(strings.ReplaceAll(iss.Fields.Summary, "\n", " "))
	room := m.modalWidth() - ansi.StringWidth(key) - 2
	b.WriteString(key)
	if room > 0 && summary != "" {
		b.WriteString("  ")
		b.WriteString(ansi.Truncate(summary, room, "…"))
	}
	b.WriteString("\n\n")
	if room < ansi.StringWidth(summary) {
		b.WriteString("Summary: ")
		b.WriteString(summary)
		b.WriteString("\n\n")
	}
	if iss.Fields.IssueType != nil {
		_, _ = fmt.Fprintf(&b, "Type:     %s\n", iss.Fields.IssueType.Name)
	}
	if iss.Fields.Status != nil {
		_, _ = fmt.Fprintf(&b, "Status:   %s\n", iss.Fields.Status.Name)
	}
	if iss.Fields.Priority != nil {
		_, _ = fmt.Fprintf(&b, "Priority: %s\n", iss.Fields.Priority.Name)
	}
	if iss.Fields.Assignee != nil {
		_, _ = fmt.Fprintf(&b, "Assignee: %s\n", iss.Fields.Assignee.DisplayName)
	} else {
		b.WriteString("Assignee: Unassigned\n")
	}
	_, _ = fmt.Fprintf(&b, "Created:  %s\n", iss.Fields.Created)
	_, _ = fmt.Fprintf(&b, "Updated:  %s\n", iss.Fields.Updated)
	if iss.Fields.Description != nil {
		b.WriteString("\n── Description ──\n")
		for _, block := range iss.Fields.Description.Content {
			for _, content := range block.Content {
				if content.Text != "" {
					b.WriteString(content.Text)
					b.WriteString("\n")
				}
			}
		}
	}
	return b.String(), "[↑↓] Scroll  [c] JSON  [e] Edit description  [w] Log  [o] Open  [esc] Close"
}
