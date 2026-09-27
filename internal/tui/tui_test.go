package tui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func testIssue(key string) jira.Issue {
	return jira.Issue{
		Key:    key,
		Fields: IssueFields("Test issue " + key),
	}
}

func IssueFields(summary string) jira.IssueFieldData {
	return jira.IssueFieldData{
		Summary:  summary,
		Status:   &jira.StatusField{Name: "To Do", ID: "10000"},
		Priority: &jira.PriorityField{Name: "High"},
		Assignee: &jira.AssigneeField{DisplayName: "TestUser"},
	}
}

func makeTestModel(cols int, issuesPerCol int) Model {
	columns := make([]column, cols)
	for c := 0; c < cols; c++ {
		col := column{
			name:   "Col" + string(rune('A'+c)),
			issues: make([]jira.Issue, issuesPerCol),
		}
		for i := 0; i < issuesPerCol; i++ {
			col.issues[i] = testIssue(strings.Repeat("P", 1) + string(rune('0'+i%10)))
		}
		columns[c] = col
	}

	return Model{
		view:     viewKanban,
		columns:  columns,
		colCur:   0,
		rowCur:   0,
		width:    190,
		height:   50,
		boardCur: 0,
		boards:   []jira.Board{{ID: 1, Name: "Test Board", Type: "scrum"}},
	}
}

func TestKanbanMultipleColumnsFit(t *testing.T) {
	m := makeTestModel(7, 5)
	cpp := m.colsPerPage()
	if cpp < 2 {
		t.Fatalf("Expected cols per page >= 2 at width=190, got %d", cpp)
	}

	output := m.View().Content

	for _, col := range m.columns {
		if !strings.Contains(output, col.name) {
			t.Errorf("Column %q not found in output", col.name)
		}
	}
}

func TestKanbanPagination(t *testing.T) {
	m := makeTestModel(11, 3)
	cpp := m.colsPerPage()
	if cpp >= 11 {
		t.Fatalf("Expected pagination (cpp=%d < 11) at width=190", cpp)
	}

	output := m.View().Content
	shown := 0
	for _, col := range m.columns {
		if strings.Contains(output, col.name) {
			shown++
		}
	}
	if shown != cpp {
		t.Errorf("Expected %d columns visible, got %d", cpp, shown)
	}

	// Move to last column
	m.colCur = 10
	output = m.View().Content
	shown = 0
	for _, col := range m.columns {
		if strings.Contains(output, col.name) {
			shown++
		}
	}
	if shown != cpp {
		t.Errorf("After paging, expected %d columns, got %d", cpp, shown)
	}
	if !strings.Contains(output, m.columns[10].name) {
		t.Error("Last column not visible after paging")
	}
}

func TestKanbanEmptyColumnSameHeight(t *testing.T) {
	full := makeTestModel(3, 10)
	empty := makeTestModel(3, 10)
	empty.columns[1].issues = nil

	fullOut := full.View().Content
	emptyOut := empty.View().Content

	fullLines := strings.Count(fullOut, "\n")
	emptyLines := strings.Count(emptyOut, "\n")

	diff := fullLines - emptyLines
	if diff < -2 || diff > 2 {
		t.Errorf("Line count mismatch: full=%d, empty=%d (diff=%d)", fullLines, emptyLines, diff)
	}
}

func TestKanbanScrollFollowsCursor(t *testing.T) {
	m := makeTestModel(1, 20)
	m.height = 30

	out1 := m.View().Content
	m.rowCur = 19
	out2 := m.View().Content

	if out1 == "" || out2 == "" {
		t.Error("Empty view output")
	}
}

func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func textKey(text string) tea.KeyPressMsg {
	runes := []rune(text)
	return tea.KeyPressMsg{Code: runes[0], Text: text}
}

func TestViewsUseAltScreen(t *testing.T) {
	mainView := makeTestModel(1, 1).View()
	if !mainView.AltScreen {
		t.Fatal("main TUI view must request the alternate screen")
	}
	if !strings.Contains(mainView.Content, "Test Board") {
		t.Fatal("main TUI content was not preserved in tea.View.Content")
	}

	setupView := NewSetup().View()
	if !setupView.AltScreen {
		t.Fatal("setup view must request the alternate screen")
	}
	if !strings.Contains(setupView.Content, "GoJira Setup") {
		t.Fatal("setup content was not preserved in tea.View.Content")
	}
}

func updateModel(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	result, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	return result, cmd
}

func boardResult(requestID uint64, boardID int, sprint string, issueKey string) boardLoadMsg {
	return boardLoadMsg{
		requestID:  requestID,
		boardID:    boardID,
		columns:    []column{{name: "To Do", issues: []jira.Issue{testIssue(issueKey)}}},
		sprintName: sprint,
		sprintID:   boardID * 10,
	}
}

func TestBoardSwitchImmediatelyHidesPreviousBoard(t *testing.T) {
	m := makeTestModel(1, 1)
	m.view = viewBoards
	m.boards = []jira.Board{
		{ID: 1, Name: "Board A", Type: "scrum"},
		{ID: 2, Name: "Board B", Type: "kanban"},
	}
	m.boardCur = 1
	m.sprintName = "Sprint A"
	m.columns[0].issues[0] = testIssue("A-1")

	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("selecting a board should start a load command")
	}
	if !m.boardLoading || m.loadingBoardName != "Board B" {
		t.Fatalf("loading state = %v for %q, want Board B", m.boardLoading, m.loadingBoardName)
	}
	if len(m.columns) != 0 || m.sprintName != "" {
		t.Fatalf("previous board data remained during load: columns=%d sprint=%q", len(m.columns), m.sprintName)
	}

	view := m.View().Content
	if !strings.Contains(view, "Loading Board B...") {
		t.Fatalf("loading view does not identify target board:\n%s", view)
	}
	for _, stale := range []string{"Sprint A", "A-1"} {
		if strings.Contains(view, stale) {
			t.Errorf("loading view contains stale board data %q:\n%s", stale, view)
		}
	}
}

func TestBoardLoadingDisablesCreateWithoutDiscardingResponse(t *testing.T) {
	m := Model{view: viewBoards, boards: []jira.Board{{ID: 1, Name: "Board A"}}}
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	requestID := m.activeBoardRequest

	m, cmd := updateModel(t, m, textKey("n"))
	if cmd != nil {
		t.Fatal("create key started a command while the board was loading")
	}
	if m.view != viewKanban || !m.boardLoading || m.activeBoardRequest != requestID {
		t.Fatalf("create key disrupted loading: view=%v loading=%v request=%d", m.view, m.boardLoading, m.activeBoardRequest)
	}

	m, _ = updateModel(t, m, boardResult(requestID, 1, "Sprint A", "A-1"))
	if m.view != viewKanban || m.boardLoading || len(m.columns) != 1 {
		t.Fatalf("matching response was not accepted after loading-time key: view=%v loading=%v columns=%d", m.view, m.boardLoading, len(m.columns))
	}
}

func TestBoardScopedAsyncResponsesIgnoreStaleBoardContext(t *testing.T) {
	tests := []struct {
		name     string
		startKey tea.KeyPressMsg
		response func(uint64) tea.Msg
	}{
		{
			name:     "detail",
			startKey: specialKey(tea.KeyEnter),
			response: func(boardRequest uint64) tea.Msg {
				issue := testIssue("A-1")
				return detailMsg{issue: &issue, issueKey: "A-1", boardRequest: boardRequest, detailRequest: 1}
			},
		},
		{
			name:     "transitions",
			startKey: textKey("t"),
			response: func(boardRequest uint64) tea.Msg {
				return transitionsMsg{
					transitions:  []jira.Transition{{ID: "2", Name: "Done"}},
					issueKey:     "A-1",
					boardRequest: boardRequest,
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{view: viewBoards, boards: []jira.Board{{ID: 1, Name: "Board A"}, {ID: 2, Name: "Board B"}}}
			m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
			requestA := m.activeBoardRequest
			m, _ = updateModel(t, m, boardResult(requestA, 1, "Sprint A", "A-1"))
			if m.loadedBoardRequest != requestA {
				t.Fatalf("loaded board context=%d, want %d", m.loadedBoardRequest, requestA)
			}

			m, cmd := updateModel(t, m, tt.startKey)
			if cmd == nil {
				t.Fatal("board action did not start its asynchronous request")
			}
			stale := tt.response(requestA)

			m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
			if m.view == viewKanban {
				m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
			}
			m, _ = updateModel(t, m, stale)
			if m.view != viewBoards {
				t.Fatalf("stale response opened view %v while browsing boards", m.view)
			}

			m, _ = updateModel(t, m, specialKey(tea.KeyDown))
			m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
			requestB := m.activeBoardRequest
			m, _ = updateModel(t, m, stale)
			if m.view != viewKanban || !m.boardLoading || m.activeBoardRequest != requestB {
				t.Fatalf("stale response disrupted Board B load: view=%v loading=%v request=%d", m.view, m.boardLoading, m.activeBoardRequest)
			}

			m, _ = updateModel(t, m, boardResult(requestB, 2, "Sprint B", "B-1"))
			if m.boardLoading || m.loadedBoardRequest != requestB {
				t.Fatalf("Board B remained stuck after matching response: loading=%v context=%d", m.boardLoading, m.loadedBoardRequest)
			}
			m, _ = updateModel(t, m, stale)
			if m.view != viewKanban || m.detail != nil || len(m.transitions) != 0 {
				t.Fatalf("stale response contaminated loaded Board B: view=%v detail=%v transitions=%d", m.view, m.detail != nil, len(m.transitions))
			}
		})
	}
}

func TestBoardScopedAsyncResponsesOpenForCurrentBoard(t *testing.T) {
	tests := []struct {
		name     string
		startKey tea.KeyPressMsg
		response func(uint64) tea.Msg
		wantView view
	}{
		{
			name:     "detail",
			startKey: specialKey(tea.KeyEnter),
			response: func(boardRequest uint64) tea.Msg {
				issue := testIssue("A-1")
				return detailMsg{issue: &issue, issueKey: "A-1", boardRequest: boardRequest, detailRequest: 1}
			},
			wantView: viewDetail,
		},
		{
			name:     "transitions",
			startKey: textKey("t"),
			response: func(boardRequest uint64) tea.Msg {
				return transitionsMsg{
					transitions:  []jira.Transition{{ID: "2", Name: "Done"}},
					issueKey:     "A-1",
					boardRequest: boardRequest,
				}
			},
			wantView: viewTransition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{view: viewBoards, boards: []jira.Board{{ID: 1, Name: "Board A"}}}
			m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
			requestID := m.activeBoardRequest
			m, _ = updateModel(t, m, boardResult(requestID, 1, "Sprint A", "A-1"))
			m, _ = updateModel(t, m, tt.startKey)
			m, _ = updateModel(t, m, tt.response(m.loadedBoardRequest))
			if m.view != tt.wantView {
				t.Fatalf("current-board response opened view %v, want %v", m.view, tt.wantView)
			}
		})
	}
}

func TestDetailOpensLoadingModalAndPreservesBoardCursor(t *testing.T) {
	m := makeTestModel(2, 2)
	m.loadedBoardRequest = 7
	m.colCur = 1
	m.rowCur = 1

	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil || m.view != viewDetail || !m.detailLoading || m.detailIssueKey != "P1" {
		t.Fatalf("detail did not open immediately: cmd=%v view=%v loading=%v key=%q", cmd != nil, m.view, m.detailLoading, m.detailIssueKey)
	}
	loading := m.View().Content
	if !strings.Contains(loading, "Loading ticket detail...") || !strings.Contains(loading, "ColA") {
		t.Fatalf("loading detail was not composed over the board:\n%s", loading)
	}

	issue := testIssue("P1")
	m, _ = updateModel(t, m, detailMsg{issue: &issue, issueKey: "P1", boardRequest: 7, detailRequest: m.activeDetailRequest})
	if m.detailLoading || m.detail == nil || m.detail.Key != "P1" {
		t.Fatalf("matching detail response was not shown: loading=%v detail=%v", m.detailLoading, m.detail)
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewKanban || m.colCur != 1 || m.rowCur != 1 {
		t.Fatalf("closing detail lost board cursor: view=%v cursor=(%d,%d)", m.view, m.colCur, m.rowCur)
	}
}

func TestDetailRejectsMismatchedIssueResponse(t *testing.T) {
	m := makeTestModel(1, 1)
	m.loadedBoardRequest = 9
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	firstRequest := m.activeDetailRequest
	wrong := testIssue("OTHER-1")
	m, _ = updateModel(t, m, detailMsg{issue: &wrong, issueKey: "OTHER-1", boardRequest: 9, detailRequest: firstRequest})
	if m.view != viewDetail || !m.detailLoading || m.detail != nil || m.detailIssueKey != "P0" {
		t.Fatalf("mismatched issue response altered active detail request: %#v", m)
	}

	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	issue := testIssue("P0")
	m, _ = updateModel(t, m, detailMsg{issue: &issue, issueKey: "P0", boardRequest: 9, detailRequest: firstRequest})
	if !m.detailLoading || m.detail != nil || m.activeDetailRequest == firstRequest {
		t.Fatal("response from an earlier open of the same ticket replaced the current request")
	}
}

func TestHelpOverlayBlocksBoardActionsAndCtrlCStillQuits(t *testing.T) {
	m := makeTestModel(2, 2)
	m.loadedBoardRequest = 3
	m, _ = updateModel(t, m, textKey("?"))
	if !m.helpOpen || !strings.Contains(m.View().Content, "Kanban Help") {
		t.Fatal("help key did not open the Kanban help overlay")
	}

	for _, key := range []tea.KeyPressMsg{specialKey(tea.KeyDown), specialKey(tea.KeyRight), textKey("n"), specialKey(tea.KeyEnter)} {
		var cmd tea.Cmd
		m, cmd = updateModel(t, m, key)
		if cmd != nil || m.view != viewKanban || m.colCur != 0 || m.rowCur != 0 || !m.helpOpen {
			t.Fatalf("help leaked key %q to board: view=%v cursor=(%d,%d) open=%v", key.String(), m.view, m.colCur, m.rowCur, m.helpOpen)
		}
	}
	_, quit := updateModel(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if quit == nil {
		t.Fatal("ctrl+c did not remain available while help was open")
	}
	m, cmd := updateModel(t, m, textKey("q"))
	if cmd != nil || m.view != viewKanban || m.helpOpen {
		t.Fatal("closing help with q triggered the underlying board quit/back action")
	}
}

func TestWorklogReturnsToDetailAndIgnoresCancelledResponse(t *testing.T) {
	issue := testIssue("P0")
	m := makeTestModel(1, 1)
	m.view = viewDetail
	m.detail = &issue
	m.detailIssueKey = issue.Key
	m.detailBoardRequest = 4
	m.loadedBoardRequest = 4

	m, _ = updateModel(t, m, textKey("w"))
	if m.view != viewWorklog {
		t.Fatalf("worklog did not remain a full-screen flow: view=%v", m.view)
	}
	m.worklogInput.SetValue("2h")
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil || m.activeWorklogRequest == 0 {
		t.Fatal("worklog submit did not start a scoped request")
	}
	requestID := m.activeWorklogRequest
	m, _ = updateModel(t, m, worklogDoneMsg{requestID: requestID, issueKey: issue.Key})
	if m.view != viewDetail || m.detail == nil || m.detail.Key != issue.Key || m.toast != "Worklog added" {
		t.Fatalf("successful worklog did not return to detail: view=%v detail=%v toast=%q", m.view, m.detail, m.toast)
	}

	m, _ = updateModel(t, m, textKey("w"))
	m.worklogInput.SetValue("30m")
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	cancelledID := m.activeWorklogRequest
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, worklogDoneMsg{requestID: cancelledID, issueKey: issue.Key, err: errors.New("late failure")})
	if m.view != viewDetail || m.activeWorklogRequest != 0 || m.err != nil || m.detail == nil {
		t.Fatalf("cancelled worklog response altered detail state: view=%v request=%d err=%v", m.view, m.activeWorklogRequest, m.err)
	}
}

func TestToastGeometryIsBoundedAfterUpdate(t *testing.T) {
	issue := testIssue("P0")
	worklog := makeTestModel(1, 1)
	worklog.width, worklog.height = 40, 8
	worklog.view = viewDetail
	worklog.detail = &issue
	worklog.detailIssueKey = issue.Key
	worklog.detailBoardRequest = 4
	worklog.loadedBoardRequest = 4
	worklog, _ = updateModel(t, worklog, textKey("w"))
	worklog.worklogInput.SetValue("2h")
	worklog, _ = updateModel(t, worklog, specialKey(tea.KeyEnter))
	worklog, _ = updateModel(t, worklog, worklogDoneMsg{
		requestID: worklog.activeWorklogRequest,
		issueKey:  issue.Key,
	})
	if worklog.toast == "" || worklog.view != viewDetail {
		t.Fatalf("worklog success did not produce detail toast: view=%v toast=%q", worklog.view, worklog.toast)
	}

	longToast := makeTestModel(2, 2)
	longToast.width, longToast.height = 28, 7
	longToast.view = viewDetail
	longToast.detail = &issue
	longToast.detailIssueKey = issue.Key
	longToast, _ = updateModel(t, longToast, toastMsg{text: strings.Repeat("long toast text ", 20)})

	normal := makeTestModel(1, 1)
	normal.width, normal.height = 60, 12
	normal, _ = updateModel(t, normal, toastMsg{text: "Board updated"})

	tiny := makeTestModel(1, 1)
	tiny.width, tiny.height = 1, 1
	tiny, _ = updateModel(t, tiny, toastMsg{text: strings.Repeat("tiny toast ", 10)})

	tests := []struct {
		name     string
		model    Model
		contains []string
	}{
		{name: "worklog success overlay", model: worklog, contains: []string{"✓", "Worklog added"}},
		{name: "long detail toast", model: longToast, contains: []string{"✓", "long toast text"}},
		{name: "normal kanban toast", model: normal, contains: []string{"Test Board", "✓", "Board updated"}},
		{name: "tiny kanban toast", model: tiny, contains: []string{"✓"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.model.View().Content
			for _, text := range tt.contains {
				if !strings.Contains(output, text) {
					t.Fatalf("%q is not visible in bounded view:\n%s", text, output)
				}
			}
			for lineNumber, line := range strings.Split(output, "\n") {
				if got := lipgloss.Width(line); got > tt.model.width {
					t.Fatalf("line %d width=%d exceeds terminal width=%d:\n%s", lineNumber+1, got, tt.model.width, output)
				}
			}
			if got := lipgloss.Height(output); got > tt.model.height {
				t.Fatalf("view height=%d exceeds terminal height=%d:\n%s", got, tt.model.height, output)
			}
		})
	}
}

func TestWorklogSuccessToastPreservesFullKanbanFooter(t *testing.T) {
	issue := testIssue("P0")
	m := makeTestModel(3, 2)
	m.width, m.height = 190, 32
	m.view = viewDetail
	m.detail = &issue
	m.detailIssueKey = issue.Key
	m.detailBoardRequest = 4
	m.loadedBoardRequest = 4

	m, _ = updateModel(t, m, textKey("w"))
	m.worklogInput.SetValue("2h")
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, worklogDoneMsg{
		requestID: m.activeWorklogRequest,
		issueKey:  issue.Key,
	})

	output := m.View().Content
	for _, want := range []string{"P0", "Worklog added", "[←→] Cols", "[?] Help", "[esc] Back"} {
		if !strings.Contains(output, want) {
			t.Fatalf("full compositor toast dropped %q:\n%s", want, output)
		}
	}
	if got := lipgloss.Height(output); got != m.height {
		t.Fatalf("composited view height=%d, want terminal height=%d:\n%s", got, m.height, output)
	}
	for lineNumber, line := range strings.Split(output, "\n") {
		if got := ansi.StringWidth(line); got > m.width {
			t.Fatalf("ANSI line %d width=%d exceeds terminal width=%d", lineNumber+1, got, m.width)
		}
	}
}

func TestKanbanToastUsesDiscoverableCompactFooter(t *testing.T) {
	m := makeTestModel(2, 2)
	m.width, m.height = 60, 12
	m.toast = "Board updated"

	output := m.View().Content
	for _, want := range []string{"[?] Help", "[esc] Back", "Board updated"} {
		if !strings.Contains(output, want) {
			t.Fatalf("compact toast view dropped %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "[←→] Cols") {
		t.Fatalf("compact footer was left-prefix clipped instead of intentionally replaced:\n%s", output)
	}
	if got := lipgloss.Height(output); got > m.height {
		t.Fatalf("compact view height=%d exceeds terminal height=%d", got, m.height)
	}
	for lineNumber, line := range strings.Split(output, "\n") {
		if got := ansi.StringWidth(line); got > m.width {
			t.Fatalf("ANSI line %d width=%d exceeds terminal width=%d", lineNumber+1, got, m.width)
		}
	}
}

func TestOverlayGeometryIsBounded(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		help   bool
	}{
		{name: "normal detail", width: 120, height: 32},
		{name: "compact detail", width: 36, height: 10},
		{name: "normal help", width: 120, height: 32, help: true},
		{name: "compact help", width: 36, height: 10, help: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := makeTestModel(3, 2)
			m.width, m.height = tt.width, tt.height
			if tt.help {
				m.helpOpen = true
			} else {
				issue := testIssue("P0")
				issue.Fields.Summary = strings.Repeat("bounded detail ", 12)
				m.view = viewDetail
				m.detail = &issue
				m.detailIssueKey = issue.Key
			}
			output := m.View().Content
			if got := lipgloss.Width(output); got > tt.width {
				t.Fatalf("overlay width=%d exceeds terminal width=%d:\n%s", got, tt.width, output)
			}
			if got := lipgloss.Height(output); got > tt.height {
				t.Fatalf("overlay height=%d exceeds terminal height=%d:\n%s", got, tt.height, output)
			}
			if tt.help && !strings.Contains(output, "Kanban Help") {
				t.Fatalf("help title missing from bounded overlay:\n%s", output)
			}
			if !tt.help && !strings.Contains(output, "P0") {
				t.Fatalf("detail key missing from bounded overlay:\n%s", output)
			}
		})
	}
}

func TestBoardLoadRejectsObsoleteResponses(t *testing.T) {
	boards := []jira.Board{{ID: 1, Name: "Board A"}, {ID: 2, Name: "Board B"}}
	m := Model{view: viewBoards, boards: boards, width: 120, height: 30}

	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	requestA := m.activeBoardRequest
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	requestB := m.activeBoardRequest

	m, _ = updateModel(t, m, boardResult(requestA, 1, "Sprint A", "A-1"))
	if !m.boardLoading || len(m.columns) != 0 {
		t.Fatal("obsolete Board A success replaced the active Board B loading state")
	}

	m, _ = updateModel(t, m, boardResult(requestB, 2, "Sprint B", "B-1"))
	if m.boardLoading || m.sprintName != "Sprint B" || m.columns[0].issues[0].Key != "B-1" {
		t.Fatalf("active Board B result was not applied: loading=%v sprint=%q", m.boardLoading, m.sprintName)
	}

	m, _ = updateModel(t, m, boardLoadMsg{requestID: requestA, boardID: 1, err: errors.New("late A failure")})
	if m.err != nil || m.sprintName != "Sprint B" {
		t.Fatalf("obsolete error replaced newer success: err=%v sprint=%q", m.err, m.sprintName)
	}
}

func TestBoardLoadFailureWinsOnlyForActiveRequest(t *testing.T) {
	m := Model{
		view:   viewBoards,
		boards: []jira.Board{{ID: 1, Name: "Board A"}, {ID: 2, Name: "Board B"}},
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	requestA := m.activeBoardRequest
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	requestB := m.activeBoardRequest

	loadErr := errors.New("Board B unavailable")
	m, _ = updateModel(t, m, boardLoadMsg{requestID: requestB, boardID: 2, err: loadErr})
	m, _ = updateModel(t, m, boardResult(requestA, 1, "Sprint A", "A-1"))

	if !errors.Is(m.err, loadErr) || m.boardLoading || len(m.columns) != 0 {
		t.Fatalf("active failure was not preserved: err=%v loading=%v columns=%d", m.err, m.boardLoading, len(m.columns))
	}
	view := m.View().Content
	if !strings.Contains(view, "Board B unavailable") || strings.Contains(view, "A-1") {
		t.Fatalf("failure view has incorrect response ordering:\n%s", view)
	}
}

func TestLeavingAndReenteringSameBoardInvalidatesPreviousRequest(t *testing.T) {
	m := Model{view: viewBoards, boards: []jira.Board{{ID: 1, Name: "Board A"}}}
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	firstRequest := m.activeBoardRequest
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.activeBoardRequest != 0 {
		t.Fatal("leaving the board did not invalidate its request")
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	secondRequest := m.activeBoardRequest
	if secondRequest <= firstRequest {
		t.Fatalf("re-entry request %d did not supersede request %d", secondRequest, firstRequest)
	}

	m, _ = updateModel(t, m, boardResult(firstRequest, 1, "Old Sprint", "A-OLD"))
	if !m.boardLoading || len(m.columns) != 0 {
		t.Fatal("pre-exit response was accepted after re-entering the same board")
	}
	m, _ = updateModel(t, m, boardResult(secondRequest, 1, "New Sprint", "A-NEW"))
	if m.sprintName != "New Sprint" || m.columns[0].issues[0].Key != "A-NEW" {
		t.Fatal("re-entry response was not accepted")
	}
}

func TestBoardMutationsStartSameBoardRefresh(t *testing.T) {
	tests := []struct {
		name      string
		msg       tea.Msg
		wantFocus string
	}{
		{name: "create", msg: createdMsg{key: "NEW-1", sprintID: 10}, wantFocus: "NEW-1"},
		{name: "transition", msg: transitionDoneMsg{issueKey: "MOVE-1"}},
		{name: "sprint", msg: sprintDoneMsg{issueKey: "SPRINT-1"}, wantFocus: "SPRINT-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := makeTestModel(1, 1)
			m.sprintName = "Sprint A"
			m.sprintID = 10
			m, cmd := updateModel(t, m, tt.msg)

			if cmd == nil || !m.boardLoading || m.activeBoardRequest == 0 {
				t.Fatalf("mutation did not start an identified refresh: cmd=%v loading=%v request=%d", cmd != nil, m.boardLoading, m.activeBoardRequest)
			}
			if m.loadingBoardID != 1 || len(m.columns) != 0 || m.sprintName != "" {
				t.Fatalf("refresh state is not clean for current board: board=%d columns=%d sprint=%q", m.loadingBoardID, len(m.columns), m.sprintName)
			}
			if m.focusIssue != tt.wantFocus {
				t.Fatalf("focusIssue=%q, want %q", m.focusIssue, tt.wantFocus)
			}
			if tt.wantFocus == "" {
				return
			}

			requestID := m.activeBoardRequest
			m, _ = updateModel(t, m, boardLoadMsg{
				requestID: requestID,
				boardID:   1,
				columns: []column{
					{name: "To Do", issues: []jira.Issue{testIssue("OTHER-1")}},
					{name: "In Progress", issues: []jira.Issue{testIssue("OTHER-2"), testIssue(tt.wantFocus)}},
				},
				sprintName: "Sprint A",
				sprintID:   10,
			})
			if m.colCur != 1 || m.rowCur != 1 {
				t.Fatalf("focused cursor=(%d,%d), want (1,1)", m.colCur, m.rowCur)
			}
			if m.focusIssue != "" {
				t.Fatalf("focusIssue=%q after matching response, want cleared", m.focusIssue)
			}
		})
	}
}

func TestPasteRoutesToActiveFormInput(t *testing.T) {
	tests := []struct {
		name      string
		model     Model
		wantValue func(Model) string
	}{
		{
			name: "create summary",
			model: func() Model {
				input := textinput.New()
				input.Focus()
				return Model{view: viewCreate, createField: 1, createSummary: input}
			}(),
			wantValue: func(m Model) string { return m.createSummary.Value() },
		},
		{
			name: "create description",
			model: func() Model {
				input := textinput.New()
				input.Focus()
				return Model{view: viewCreate, createField: 2, createDesc: input}
			}(),
			wantValue: func(m Model) string { return m.createDesc.Value() },
		},
		{
			name: "worklog",
			model: func() Model {
				input := textinput.New()
				input.Focus()
				return Model{view: viewWorklog, worklogInput: input}
			}(),
			wantValue: func(m Model) string { return m.worklogInput.Value() },
		},
	}

	const pasted = "nq 2h pasted"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := updateModel(t, tt.model, tea.PasteMsg{Content: pasted})
			if got := tt.wantValue(m); got != pasted {
				t.Fatalf("pasted value=%q, want %q", got, pasted)
			}
			if m.view != tt.model.view {
				t.Fatalf("paste changed view from %v to %v", tt.model.view, m.view)
			}
		})
	}
}

func TestPasteOutsideFormsDoesNotAlterState(t *testing.T) {
	tests := []struct {
		name string
		view view
	}{
		{name: "boards", view: viewBoards},
		{name: "kanban", view: viewKanban},
		{name: "detail", view: viewDetail},
		{name: "transition", view: viewTransition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := makeTestModel(1, 1)
			before.view = tt.view
			after, cmd := updateModel(t, before, tea.PasteMsg{Content: "nq"})
			if cmd != nil {
				t.Fatal("paste outside a form returned a command")
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("paste outside a form altered model state: before=%#v after=%#v", before, after)
			}
		})
	}
}
