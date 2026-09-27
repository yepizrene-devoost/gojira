package tui

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/x/ansi"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
)

type column struct {
	name   string
	issues []jira.Issue
}

// Model is the top-level Bubble Tea model for GoJira's TUI.
// Kept as a single model with NO sub-model message routing.
type Model struct {
	client       *jira.Client
	domain       string
	view         view
	boards       []jira.Board
	boardCur     int
	columns      []column
	colCur       int
	rowCur       int
	sprintName   string
	width        int
	height       int
	err          error
	detail       *jira.Issue
	toast        string
	transitions  []jira.Transition
	transCur     int
	transIssue   string
	worklogInput textinput.Model
	worklogIssue string

	// Active sprint of the current board (0 = none; used by create + "s")
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
	issue        *jira.Issue
	boardRequest uint64
	err          error
}
type clipboardMsg struct{ text string; err error }
type toastMsg struct{ text string }
type transitionsMsg struct {
	transitions  []jira.Transition
	issueKey     string
	boardRequest uint64
	err          error
}
type transitionDoneMsg struct{ issueKey string; err error }
type worklogDoneMsg struct{ err error }
type createTypesMsg struct{ types []string; err error }
type createdMsg struct{ key string; sprintID int; err error }
type sprintDoneMsg struct{ issueKey string; err error }
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
	m.err = nil
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
	// reachable: select one and press "s" to move it into the active sprint.
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
		return m, nil

	case boardsMsg:
		m.boards = msg.boards
		return m, nil

	case boardLoadMsg:
		if m.view != viewKanban || msg.requestID != m.activeBoardRequest || msg.boardID != m.loadingBoardID {
			return m, nil
		}
		m.activeBoardRequest = 0
		m.boardLoading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.loadedBoardRequest = msg.requestID
		m.columns = msg.columns
		m.sprintName = msg.sprintName
		m.sprintID = msg.sprintID
		m.err = nil
		if m.focusIssue != "" {
			if c, r, ok := findIssuePos(msg.columns, m.focusIssue); ok {
				m.colCur, m.rowCur = c, r
			}
			m.focusIssue = ""
		}
		return m, nil

	case detailMsg:
		if m.view != viewKanban || m.boardLoading || msg.boardRequest == 0 || msg.boardRequest != m.loadedBoardRequest {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.detail = msg.issue
		m.view = viewDetail
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
		if m.view != viewKanban || m.boardLoading || msg.boardRequest == 0 || msg.boardRequest != m.loadedBoardRequest {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.transitions = msg.transitions
		m.transCur = 0
		m.transIssue = msg.issueKey
		m.view = viewTransition
		return m, nil

	case transitionDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.view = viewKanban
			return m, nil
		}
		m.toast = fmt.Sprintf("Moved %s", msg.issueKey)
		m.view = viewKanban
		m.colCur = 0
		m.rowCur = 0
		if m.boardCur < len(m.boards) {
			loaded, cmd := m.startBoardLoad(m.boards[m.boardCur])
			return loaded, cmd
		}
		return m, nil

	case worklogDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.view = viewKanban
			return m, nil
		}
		m.toast = "Worklog added"
		m.view = viewDetail
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

	case tea.KeyMsg:
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
		}
	}
	return m, nil
}

func (m Model) updateBoards(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	}
	return m, nil
}

func (m Model) updateKanban(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.boardLoading && key != "ctrl+c" && key != "q" && key != "esc" {
		return m, nil
	}

	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "q", "esc":
		m.view = viewBoards
		m.colCur = 0
		m.rowCur = 0
		m.activeBoardRequest = 0
		m.loadedBoardRequest = 0
		m.boardLoading = false
		return m, nil
	case "left", "h":
		if m.colCur > 0 {
			m.colCur--
			m.rowCur = 0
		}
	case "right", "l":
		if m.colCur < len(m.columns)-1 {
			m.colCur++
			m.rowCur = 0
		}
	case "up", "k":
		if m.rowCur > 0 {
			m.rowCur--
		}
	case "down", "j":
		if m.colCur < len(m.columns) && m.rowCur < len(m.columns[m.colCur].issues)-1 {
			m.rowCur++
		}
	case "enter":
		if m.colCur < len(m.columns) {
			issues := m.columns[m.colCur].issues
			if m.rowCur >= 0 && m.rowCur < len(issues) {
				key := issues[m.rowCur].Key
				boardRequest := m.loadedBoardRequest
				return m, func() tea.Msg {
					iss, err := m.client.GetIssue(key)
					return detailMsg{issue: iss, boardRequest: boardRequest, err: err}
				}
			}
		}
	case "o":
		if m.colCur < len(m.columns) {
			issues := m.columns[m.colCur].issues
			if m.rowCur >= 0 && m.rowCur < len(issues) {
				return m, openBrowser(issues[m.rowCur].Key, m.domain)
			}
		}
	case "C":
		if m.colCur < len(m.columns) {
			col := m.columns[m.colCur]
			tickets := jira.IssuesToTicketJSON(col.issues, m.domain)
			b, _ := json.MarshalIndent(tickets, "", "  ")
			return m, copyToClipboard(string(b))
		}
	case "t":
		if m.colCur < len(m.columns) {
			issues := m.columns[m.colCur].issues
			if m.rowCur >= 0 && m.rowCur < len(issues) {
				key := issues[m.rowCur].Key
				boardRequest := m.loadedBoardRequest
				return m, func() tea.Msg {
					tr, err := m.client.GetTransitions(key)
					return transitionsMsg{transitions: tr, issueKey: key, boardRequest: boardRequest, err: err}
				}
			}
		}
	case "n":
		return m.enterCreate()
	case "s":
		if m.colCur >= len(m.columns) {
			return m, nil
		}
		issues := m.columns[m.colCur].issues
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

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "q":
		m.view = viewKanban
		m.detail = nil
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
			m.view = viewWorklog
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m Model) updateTransition(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
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
		if m.transCur < len(m.transitions) {
			tr := m.transitions[m.transCur]
			key := m.transIssue
			return m, func() tea.Msg {
				err := m.client.TransitionIssue(key, tr.ID)
				return transitionDoneMsg{issueKey: key, err: err}
			}
		}
	}
	return m, nil
}

func (m Model) updateWorklog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewDetail
		return m, nil
	case "enter":
		timeSpent := m.worklogInput.Value()
		if timeSpent == "" {
			return m, nil
		}
		key := m.worklogIssue
		return m, func() tea.Msg {
			err := m.client.AddWorklog(key, timeSpent, "")
			return worklogDoneMsg{err: err}
		}
	default:
		var cmd tea.Cmd
		m.worklogInput, cmd = m.worklogInput.Update(msg)
		return m, cmd
	}
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
	sum.Width = 40
	desc := textinput.New()
	desc.Placeholder = "optional, single line"
	desc.CharLimit = 2000
	desc.Width = 40

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

func (m Model) updateCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m Model) View() string {
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
			return b.String()
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
		b.WriteString(subStyle.Render("[↑↓] Navigate  [Enter] Select  [q] Quit"))

	case viewKanban:
		boardName := m.loadingBoardName
		if !m.boardLoading && m.boardCur >= 0 && m.boardCur < len(m.boards) {
			boardName = m.boards[m.boardCur].Name
		}
		b.WriteString(titleStyle.Render(fmt.Sprintf("📋 %s", boardName)))
		if !m.boardLoading {
			b.WriteString(" ")
			b.WriteString(subStyle.Render(fmt.Sprintf("Sprint: %s", m.sprintName)))
		}
		b.WriteString("\n")

		if m.boardLoading {
			b.WriteString("\n")
			b.WriteString(subStyle.Render("Loading " + boardName + "..."))
			return b.String()
		}

		if m.err != nil {
			b.WriteString("\n")
			b.WriteString(errStyle.Render("Error: " + m.err.Error()))
			return b.String()
		}

		if len(m.columns) == 0 {
			b.WriteString(subStyle.Render("No board columns available."))
			return b.String()
		}

		cpp := m.colsPerPage()
		if cpp < 1 {
			cpp = 1
		}
		pageStart := visiblePageStart(m.colCur, cpp, len(m.columns))
		pageEnd := pageStart + cpp
		if pageEnd > len(m.columns) {
			pageEnd = len(m.columns)
		}

		colH := m.height - 4
		if colH < 5 {
			colH = 5
		}
		slots := (colH - 1) / 4

		cw := colWidth + colGap
		colStrs := make([]string, 0, pageEnd-pageStart)
		for i := pageStart; i < pageEnd; i++ {
			colStrs = append(colStrs, m.renderColumn(m.columns[i], i == m.colCur, cw, colH, slots))
		}

		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, colStrs...))
		b.WriteString("\n\n")

		totalPages := (len(m.columns) + cpp - 1) / cpp
		curPage := pageStart/cpp + 1
		b.WriteString(subStyle.Render(
			fmt.Sprintf("[←→] Cols  [↑↓] Issues  [Enter] Detail  [t] Transition  [n] New  [s] To Sprint  [C] Copy Col  [esc] Back    Col %d/%d  Page %d/%d",
				m.colCur+1, len(m.columns), curPage, totalPages)))

	case viewDetail:
		if m.detail != nil {
			b.WriteString(m.renderDetail())
		}

	case viewTransition:
		b.WriteString(titleStyle.Render("🔄 Transition: " + m.transIssue))
		b.WriteString("\n\n")
		for i, tr := range m.transitions {
			if i == m.transCur {
				b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true).Render("→ "+tr.Name))
			} else {
				b.WriteString("  " + tr.Name)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(subStyle.Render("[↑↓] Navigate  [Enter] Confirm  [esc] Cancel"))

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
	}

	// Toast
	if m.toast != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Bold(true).Render("  ✓ " + m.toast))
	}

	return b.String()
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
	iss := m.detail
	var b strings.Builder

	b.WriteString(keyStyle.Render(fmt.Sprintf("📋 %s", iss.Key)))
	b.WriteString("\n\n")
	b.WriteString(iss.Fields.Summary)
	b.WriteString("\n\n")

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
			for _, c := range block.Content {
				if c.Text != "" {
					b.WriteString(c.Text)
					b.WriteString("\n")
				}
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(subStyle.Render("[c] Copy JSON  [w] Add Worklog  [o] Browser  [esc] Close"))
	return detailBox.Render(b.String())
}
