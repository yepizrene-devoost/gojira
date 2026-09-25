package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

// ─── Styles ────────────────────────────────────────────────────────────
const colWidth = 24 // content width per column
const colGap = 2   // separator width between columns

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

// ─── API Types ─────────────────────────────────────────────────────────
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type BoardConfig struct {
	ColumnConfig *struct {
		Columns []struct {
			Name     string `json:"name"`
			Statuses []struct {
				ID string `json:"id"`
			} `json:"statuses"`
		} `json:"columns"`
	} `json:"columnConfig,omitempty"`
}

type Sprint struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

type StatusField struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}
type PriorityField struct {
	Name string `json:"name"`
}
type AssigneeField struct {
	DisplayName string `json:"displayName"`
}
type IssueTypeField struct {
	Name string `json:"name"`
}
type DescriptionField struct {
	Content []struct {
		Content []struct{ Text string } `json:"content"`
	} `json:"content"`
}
type IssueFieldData struct {
	Summary     string            `json:"summary"`
	Status      *StatusField      `json:"status,omitempty"`
	Priority    *PriorityField    `json:"priority,omitempty"`
	Assignee    *AssigneeField    `json:"assignee,omitempty"`
	IssueType   *IssueTypeField   `json:"issuetype,omitempty"`
	Created     string            `json:"created"`
	Updated     string            `json:"updated"`
	Description *DescriptionField `json:"description,omitempty"`
}
type Issue struct {
	Key    string         `json:"key"`
	Fields IssueFieldData `json:"fields"`
}

// ─── API Client ────────────────────────────────────────────────────────
type JiraClient struct {
	baseURL, auth string
	http          *http.Client
}

func NewJiraClient(domain, email, token string) *JiraClient {
	return &JiraClient{
		baseURL: "https://" + domain,
		auth:    email + ":" + token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *JiraClient) get(path string) ([]byte, error) {
	req, _ := http.NewRequest("GET", c.baseURL+path, nil)
	req.SetBasicAuth(c.auth, "")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API %d: %s", resp.StatusCode, string(body[:minInt(len(body), 200)]))
	}
	return body, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *JiraClient) TestConnection() (string, error) {
	b, err := c.get("/rest/api/3/serverInfo")
	if err != nil {
		return "", err
	}
	var info struct{ BaseURL, Version string }
	json.Unmarshal(b, &info)
	return fmt.Sprintf("%s (Jira %s)", info.BaseURL, info.Version), nil
}

func (c *JiraClient) GetBoards() ([]Board, error) {
	b, err := c.get("/rest/agile/1.0/board?maxResults=50")
	if err != nil {
		return nil, err
	}
	var r struct{ Values []Board }
	json.Unmarshal(b, &r)
	return r.Values, nil
}

func (c *JiraClient) GetBoardConfig(id int) (BoardConfig, error) {
	b, err := c.get(fmt.Sprintf("/rest/agile/1.0/board/%d/configuration", id))
	if err != nil {
		return BoardConfig{}, err
	}
	var cfg BoardConfig
	json.Unmarshal(b, &cfg)
	return cfg, nil
}

func (c *JiraClient) GetSprints(boardID int) ([]Sprint, error) {
	b, err := c.get(fmt.Sprintf("/rest/agile/1.0/board/%d/sprint?state=active&maxResults=50", boardID))
	if err != nil {
		return nil, err
	}
	var r struct{ Values []Sprint }
	json.Unmarshal(b, &r)
	if len(r.Values) > 0 {
		return r.Values, nil
	}
	b, err = c.get(fmt.Sprintf("/rest/agile/1.0/board/%d/sprint?maxResults=50", boardID))
	if err != nil {
		return nil, err
	}
	json.Unmarshal(b, &r)
	return r.Values, nil
}

func (c *JiraClient) GetBoardIssues(boardID, sprintID int) ([]Issue, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/issue?maxResults=100&fields=summary,status,priority,assignee,issuetype", boardID)
	if sprintID > 0 {
		path = fmt.Sprintf("/rest/agile/1.0/board/%d/sprint/%d/issue?maxResults=100&fields=summary,status,priority,assignee,issuetype", boardID, sprintID)
	}
	b, err := c.get(path)
	if err != nil {
		return nil, err
	}
	var r struct{ Issues []Issue }
	json.Unmarshal(b, &r)
	return r.Issues, nil
}

// ─── TUI Types ─────────────────────────────────────────────────────────
type view int

const (
	viewBoards view = iota
	viewKanban
	viewDetail
)

type column struct {
	name   string
	issues []Issue
}

type model struct {
	client     *JiraClient
	view       view
	boards     []Board
	boardCur   int
	columns    []column
	colCur     int
	rowCur     int
	sprintName string
	width      int
	height     int
	err        error
	detail     *Issue
}

// ─── Init ──────────────────────────────────────────────────────────────
func (m model) Init() tea.Cmd {
	return m.loadBoards
}

func (m model) loadBoards() tea.Msg {
	boards, err := m.client.GetBoards()
	if err != nil {
		return errMsg{err}
	}
	return boardsMsg{boards}
}

func (m model) loadBoardData(board Board) tea.Msg {
	cfg, err := m.client.GetBoardConfig(board.ID)
	if err != nil {
		return errMsg{err}
	}

	sprints, err := m.client.GetSprints(board.ID)
	if err != nil {
		return errMsg{err}
	}

	var activeSprint *Sprint
	for i, s := range sprints {
		if s.State == "active" {
			activeSprint = &sprints[i]
			break
		}
	}

	var issues []Issue
	sprintName := "All issues"
	if activeSprint != nil {
		sprintName = activeSprint.Name
		issues, err = m.client.GetBoardIssues(board.ID, activeSprint.ID)
	} else {
		issues, err = m.client.GetBoardIssues(board.ID, 0)
	}
	if err != nil {
		return errMsg{err}
	}

	// Build status-to-column mapping
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

	// Distribute issues
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
		cols = append(cols,
			column{name: "To Do", issues: issues},
		)
	}

	return kanbanMsg{columns: cols, sprintName: sprintName}
}

// ─── Messages ──────────────────────────────────────────────────────────
type boardsMsg struct{ boards []Board }
type kanbanMsg struct {
	columns    []column
	sprintName string
}
type detailMsg struct{ issue *Issue }
type errMsg struct{ err error }

// ─── Update ────────────────────────────────────────────────────────────
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case boardsMsg:
		m.boards = msg.boards
		return m, nil

	case kanbanMsg:
		m.columns = msg.columns
		m.sprintName = msg.sprintName
		return m, nil

	case detailMsg:
		m.detail = msg.issue
		m.view = viewDetail
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		switch m.view {
		case viewBoards:
			return m.updateBoards(msg)
		case viewKanban:
			return m.updateKanban(msg)
		case viewDetail:
			return m.updateDetail(msg)
		}
	}
	return m, nil
}

func (m model) updateBoards(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			m.colCur = 0
			m.rowCur = 0
			return m, func() tea.Msg { return m.loadBoardData(board) }
		}
	}
	return m, nil
}

func (m model) updateKanban(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q", "esc":
		m.view = viewBoards
		m.colCur = 0
		m.rowCur = 0
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
				return m, func() tea.Msg {
					b, err := m.client.get(fmt.Sprintf("/rest/api/3/issue/%s?fields=summary,status,priority,assignee,issuetype,created,updated,description", key))
					if err != nil {
						return errMsg{err}
					}
					var full Issue
					json.Unmarshal(b, &full)
					return detailMsg{issue: &full}
				}
			}
		}
	case "o":
		if m.colCur < len(m.columns) {
			issues := m.columns[m.colCur].issues
			if m.rowCur >= 0 && m.rowCur < len(issues) {
				key := issues[m.rowCur].Key
				return m, openBrowser(key)
			}
		}
	}
	return m, nil
}

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "q":
		m.view = viewKanban
		m.detail = nil
		return m, nil
	case "o":
		if m.detail != nil {
			return m, openBrowser(m.detail.Key)
		}
	}
	return m, nil
}

func openBrowser(key string) tea.Cmd {
	return func() tea.Msg {
		domain := os.Getenv("JIRA_DOMAIN")
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
			cmd.Start()
		}
		return nil
	}
}

// ─── View ──────────────────────────────────────────────────────────────
func (m model) View() string {
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
			if board.Type == "scrum" {
				icon = "🏉"
			} else if board.Type == "kanban" {
				icon = "📊"
			}
			line := fmt.Sprintf("%s %s [%s]", icon, board.Name, board.Type)
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
		b.WriteString(titleStyle.Render(fmt.Sprintf("📋 %s", m.boards[m.boardCur].Name)))
		b.WriteString(" ")
		b.WriteString(subStyle.Render(fmt.Sprintf("Sprint: %s", m.sprintName)))
		b.WriteString("\n")

		if m.err != nil {
			b.WriteString("\n")
			b.WriteString(errStyle.Render("Error: " + m.err.Error()))
			return b.String()
		}

		if len(m.columns) == 0 {
			b.WriteString(subStyle.Render("Loading..."))
			return b.String()
		}

		// Pagination: compute how many columns fit
		cpp := m.colsPerPage()
		if cpp < 1 {
			cpp = 1
		}
		pageStart := visiblePageStart(m.colCur, cpp, len(m.columns))
		pageEnd := pageStart + cpp
		if pageEnd > len(m.columns) {
			pageEnd = len(m.columns)
		}

		// Fixed column height from terminal
		colH := m.height - 4 // title(1) + blank(1) + hints(1) + blank(1) margin
		if colH < 5 {
			colH = 5
		}
		slots := (colH - 1) / 4 // 1 header line, 4 lines per card slot (3 card + 1 gap)

		// Build each visible column as exactly colH lines, each exactly colWidth+colGap wide
		cw := colWidth + colGap
		colStrs := make([]string, 0, pageEnd-pageStart)
		for i := pageStart; i < pageEnd; i++ {
			isActive := i == m.colCur
			colStrs = append(colStrs, m.renderColumn(m.columns[i], isActive, cw, colH, slots))
		}

		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, colStrs...))
		b.WriteString("\n\n")

		totalPages := (len(m.columns) + cpp - 1) / cpp
		curPage := pageStart/cpp + 1
		b.WriteString(subStyle.Render(
			fmt.Sprintf("[←→] Columns  [↑↓] Issues  [Enter] Detail  [o] Browser  [esc] Back    Col %d/%d  Page %d/%d",
				m.colCur+1, len(m.columns), curPage, totalPages)))

	case viewDetail:
		if m.detail != nil {
			b.WriteString(m.renderDetail())
		}
	}

	return b.String()
}

func (m model) colsPerPage() int {
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
	// Keep colCur visible in [start, start+colsPerPage)
	if colCur >= start+colsPerPage {
		start = colCur - colsPerPage + 1
	}
	if colCur < start {
		start = colCur
	}
	return start
}

func (m model) renderColumn(col column, isActive bool, cw int, colH int, slots int) string {
	lines := make([]string, 0, colH)

	// ── Header line (line 0) ──
	hdr := fmt.Sprintf(" %s (%d) ", truncateRunes(col.name, cw-6), len(col.issues))
	hdr = padRight(hdr, cw)
	hdr = colHeaderSty.Render(hdr)
	lines = append(lines, hdr)

	// ── Scroll offset: follow cursor only for active column ──
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

	// ── Card slots ──
	for i := 0; i < slots; i++ {
		idx := offset + i
		if idx < len(col.issues) {
			iss := col.issues[idx]
			sel := isActive && idx == m.rowCur
			cardLines := m.renderCardLines(iss, sel, cw)
			lines = append(lines, cardLines...)
		} else {
			// Empty slot = 3 blank lines + 1 gap
			lines = append(lines, padRight("", cw))
			lines = append(lines, padRight("", cw))
			lines = append(lines, padRight("", cw))
		}
		lines = append(lines, padRight("", cw)) // gap line after each slot
	}

	// ── Pad to exact colH ──
	for len(lines) < colH {
		lines = append(lines, padRight("", cw))
	}
	// Truncate if somehow too tall
	if len(lines) > colH {
		lines = lines[:colH]
	}

	return strings.Join(lines, "\n")
}

func (m model) renderCardLines(iss Issue, selected bool, cw int) []string {
	// Line 0: KEY
	key := truncateRunes(iss.Key, cw-2)
	keyLine := keyStyle.Render(" " + key)
	if selected {
		keyLine = lipgloss.NewStyle().Background(selBg).Render(padRight(keyLine, cw))
	} else {
		keyLine = padRight(keyLine, cw)
	}

	// Line 1: Summary (plain text, truncated, then styled)
	summary := truncateRunes(iss.Fields.Summary, cw-4)
	summaryLine := " " + summary
	if selected {
		summaryLine = padRight(summaryLine, cw)
		summaryLine = lipgloss.NewStyle().Background(selBg).Render(summaryLine)
	} else {
		summaryLine = padRight(summaryLine, cw)
	}

	// Line 2: Priority • Assignee
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

func (m model) renderDetail() string {
	iss := m.detail
	var b strings.Builder

	b.WriteString(keyStyle.Render(fmt.Sprintf("📋 %s", iss.Key)))
	b.WriteString("\n\n")
	b.WriteString(iss.Fields.Summary)
	b.WriteString("\n\n")

	if iss.Fields.IssueType != nil {
		b.WriteString(fmt.Sprintf("Type:     %s\n", iss.Fields.IssueType.Name))
	}
	if iss.Fields.Status != nil {
		b.WriteString(fmt.Sprintf("Status:   %s\n", iss.Fields.Status.Name))
	}
	if iss.Fields.Priority != nil {
		b.WriteString(fmt.Sprintf("Priority: %s\n", iss.Fields.Priority.Name))
	}
	if iss.Fields.Assignee != nil {
		b.WriteString(fmt.Sprintf("Assignee: %s\n", iss.Fields.Assignee.DisplayName))
	} else {
		b.WriteString("Assignee: Unassigned\n")
	}
	b.WriteString(fmt.Sprintf("Created:  %s\n", iss.Fields.Created))
	b.WriteString(fmt.Sprintf("Updated:  %s\n", iss.Fields.Updated))

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
	b.WriteString(subStyle.Render("[Enter] Close  [o] Browser  [esc] Close"))
	return detailBox.Render(b.String())
}

// ─── Main ──────────────────────────────────────────────────────────────
func main() {
	_ = godotenv.Load()

	email := os.Getenv("JIRA_EMAIL")
	token := os.Getenv("JIRA_API_TOKEN")
	domain := os.Getenv("JIRA_DOMAIN")

	if email == "" || token == "" || domain == "" {
		fmt.Fprintln(os.Stderr, "Error: Set JIRA_EMAIL, JIRA_API_TOKEN, JIRA_DOMAIN in .env")
		os.Exit(1)
	}

	client := NewJiraClient(domain, email, token)

	fmt.Println("Connecting to Jira...")
	info, err := client.TestConnection()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Connection failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Connected to %s\n", info)

	m := model{client: client, view: viewBoards}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
