package tui

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func startSearchModel(t *testing.T, m Model, query string) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.startSearch(query)
	result, ok := updated.(Model)
	if !ok {
		t.Fatalf("startSearch returned %T, want tui.Model", updated)
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
			if result, ok := stale.(transitionsMsg); ok {
				result.requestID = m.activeTransitionRequest
				stale = result
			}

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
			response := tt.response(m.loadedBoardRequest)
			if result, ok := response.(transitionsMsg); ok {
				result.requestID = m.activeTransitionRequest
				response = result
			}
			m, _ = updateModel(t, m, response)
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

func TestDetailModalKeepsBordersAndScrollsLongDescription(t *testing.T) {
	m := makeTestModel(8, 2)
	m.width, m.height = 190, 32
	m.view = viewDetail
	issue := testIssue("ARA-1858")
	issue.Fields.Summary = strings.Repeat("Long issue title ", 12)
	issue.Fields.Description = &jira.ADFDoc{Content: []jira.ADFNode{{
		Type: "paragraph", Content: []jira.ADFNode{{Type: "text", Text: strings.Repeat("long description words ", 120) + "END MARKER"}},
	}}}
	m.detail = &issue
	m.detailIssueKey = issue.Key

	check := func(output string) {
		t.Helper()
		if got := lipgloss.Height(output); got > m.height {
			t.Fatalf("modal height=%d exceeds terminal height=%d", got, m.height)
		}
		for _, line := range strings.Split(output, "\n") {
			if got := ansi.StringWidth(line); got > m.width {
				t.Fatalf("modal line width=%d exceeds terminal width=%d", got, m.width)
			}
		}
		plain := ansi.Strip(output)
		if !strings.Contains(plain, "╭") || !strings.Contains(plain, "╮") || !strings.Contains(plain, "╰") || !strings.Contains(plain, "╯") {
			t.Fatalf("modal border was clipped:\n%s", plain)
		}
		if !strings.Contains(plain, "[esc] Close") {
			t.Fatalf("modal actions were clipped:\n%s", plain)
		}
	}
	check(m.View().Content)
	if strings.Contains(ansi.Strip(m.View().Content), "END MARKER") {
		t.Fatal("long description should require scrolling")
	}
	for i := 0; i < 200; i++ {
		m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	}
	output := m.View().Content
	check(output)
	if !strings.Contains(ansi.Strip(output), "END MARKER") {
		t.Fatalf("last description line is unreachable:\n%s", ansi.Strip(output))
	}
	before := m.detailScroll
	m, _ = updateModel(t, m, specialKey(tea.KeyUp))
	if m.detailScroll != before-1 || m.View().Content == output {
		t.Fatalf("up after last page did not move viewport: offset %d → %d", before, m.detailScroll)
	}
}

func TestDetailScrollRepeatedLinesAndResize(t *testing.T) {
	m := makeTestModel(3, 2)
	m.width, m.height = 65, 16
	m.view = viewDetail
	issue := testIssue("P0")
	issue.Fields.Description = &jira.ADFDoc{Content: []jira.ADFNode{{
		Type: "paragraph", Content: []jira.ADFNode{{Type: "text", Text: strings.Repeat("repeat\n", 100) + "END"}},
	}}}
	m.detail = &issue
	m.detailIssueKey = issue.Key
	for i := 0; i < 110; i++ {
		m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "END") {
		t.Fatal("repeated lines blocked navigation to the final line")
	}
	if m.detailScroll != m.detailMaxScroll() {
		t.Fatalf("scroll offset %d should clamp to %d", m.detailScroll, m.detailMaxScroll())
	}
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.detailScroll != m.detailMaxScroll() {
		t.Fatalf("resize kept stale scroll offset %d, limit %d", m.detailScroll, m.detailMaxScroll())
	}
	before := m.detailScroll
	m, _ = updateModel(t, m, specialKey(tea.KeyUp))
	if m.detailScroll != before-1 {
		t.Fatalf("up after resize moved offset %d to %d", before, m.detailScroll)
	}
}

func TestCompactDetailKeepsScrollableBody(t *testing.T) {
	for _, width := range []int{36, 60} {
		m := makeTestModel(2, 2)
		m.width, m.height = width, 9
		m.view = viewDetail
		issue := testIssue("P0")
		issue.Fields.Description = &jira.ADFDoc{Content: []jira.ADFNode{{
			Type: "paragraph", Content: []jira.ADFNode{{Type: "text", Text: strings.Repeat("line\n", 20) + "END"}},
		}}}
		m.detail = &issue
		m.detailIssueKey = issue.Key
		for i := 0; i < 80; i++ {
			m, _ = updateModel(t, m, specialKey(tea.KeyDown))
		}
		plain := ansi.Strip(m.View().Content)
		if !strings.Contains(plain, "END") || !strings.Contains(plain, "[esc] Close") {
			t.Fatalf("compact viewport hid its body or close action:\n%s", plain)
		}
		if !strings.Contains(plain, "╭") || !strings.Contains(plain, "╮") || !strings.Contains(plain, "╰") || !strings.Contains(plain, "╯") {
			t.Fatalf("compact width %d lost border corners:\n%s", width, plain)
		}
	}
}

func TestModalHeightStaysConsistentAcrossContentAndTerminalSizes(t *testing.T) {
	for _, terminal := range []struct{ width, height, want int }{{190, 50, 35}, {120, 32, 30}, {60, 16, 14}} {
		t.Run(fmt.Sprintf("%dx%d", terminal.width, terminal.height), func(t *testing.T) {
			m := makeTestModel(2, 2)
			m.width, m.height = terminal.width, terminal.height
			issue := testIssue("P0")
			m.view = viewDetail
			m.detail = &issue
			m.detailIssueKey = issue.Key
			short := m.renderDetail()
			issue.Fields.Description = &jira.ADFDoc{Content: []jira.ADFNode{{Type: "paragraph", Content: []jira.ADFNode{{Type: "text", Text: strings.Repeat("many words ", 200)}}}}}
			m.detail = &issue
			long := m.renderDetail()
			m.view = viewKanban
			m.helpOpen = true
			help := m.renderHelp()
			for name, overlay := range map[string]string{"short": short, "long": long, "help": help} {
				if got := lipgloss.Height(overlay); got != terminal.want {
					t.Errorf("%s height=%d, want consistent height %d", name, got, terminal.want)
				}
				plain := ansi.Strip(overlay)
				if !strings.Contains(plain, "╭") || !strings.Contains(plain, "╮") || !strings.Contains(plain, "╰") || !strings.Contains(plain, "╯") {
					t.Errorf("%s border clipped:\n%s", name, plain)
				}
			}
		})
	}
}

func TestDetailHeaderCombinesKeyAndSummaryWithoutLosingLongSummary(t *testing.T) {
	m := makeTestModel(2, 2)
	m.width, m.height = 120, 40
	m.view = viewDetail
	issue := testIssue("ARA-1901")
	issue.Fields.Summary = "Review unexpected error sentry"
	m.detail = &issue
	content, _ := m.detailContent()
	plain := ansi.Strip(content)
	if !strings.HasPrefix(plain, "📋 ARA-1901  Review unexpected error sentry\n") || strings.Count(plain, issue.Fields.Summary) != 1 {
		t.Fatalf("short summary not in single-line title:\n%s", plain)
	}
	if got := lipgloss.Width(m.renderDetail()); got != 100 {
		t.Fatalf("modal width=%d, want 100", got)
	}

	issue.Fields.Summary = strings.Repeat("long summary words ", 12) + "FINAL"
	m.detail = &issue
	content, _ = m.detailContent()
	plain = ansi.Strip(content)
	lines := strings.Split(plain, "\n")
	if ansi.StringWidth(lines[0]) > m.modalWidth() || !strings.Contains(lines[0], "ARA-1901") || !strings.Contains(plain, issue.Fields.Summary) {
		t.Fatalf("long summary lost key, exceeded title width, or became inaccessible:\n%s", plain)
	}
}

func TestDetailPageKeysAdvanceByVisibleRows(t *testing.T) {
	m := makeTestModel(2, 2)
	m.width, m.height = 120, 50
	m.view = viewDetail
	issue := testIssue("P0")
	issue.Fields.Description = &jira.ADFDoc{Content: []jira.ADFNode{{Type: "paragraph", Content: []jira.ADFNode{{Type: "text", Text: strings.Repeat("line\n", 100)}}}}}
	m.detail = &issue
	m.detailIssueKey = issue.Key
	page := m.detailPageSize()
	if page < 1 || page > m.modalHeight()-5 {
		t.Fatalf("unexpected visible page size %d", page)
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyPgDown))
	if m.detailScroll != page {
		t.Fatalf("PgDn advanced %d rows, want %d", m.detailScroll, page)
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyPgUp))
	if m.detailScroll != 0 {
		t.Fatalf("PgUp did not return to start: %d", m.detailScroll)
	}
}

func TestCompactHelpScrollKeepsBoardCursor(t *testing.T) {
	m := makeTestModel(3, 2)
	m.width, m.height = 60, 16
	m.helpOpen = true
	col, row := m.colCur, m.rowCur
	for i := 0; i < 20; i++ {
		m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	}
	if m.colCur != col || m.rowCur != row {
		t.Fatalf("help scrolling moved board cursor: (%d,%d)", m.colCur, m.rowCur)
	}
	plain := ansi.Strip(m.renderHelp())
	if !strings.Contains(plain, "Ctrl+C") || !strings.Contains(plain, "[? / esc / q] Close") {
		t.Fatalf("compact help hid final entry or close action:\n%s", plain)
	}
}

func TestLocalFilterMatchesKeyAndSummaryCaseInsensitively(t *testing.T) {
	m := makeTestModel(2, 0)
	m.columns[0].issues = []jira.Issue{
		{Key: "ARA-101", Fields: IssueFields("Fix Login Flow")},
		{Key: "OPS-7", Fields: IssueFields("Rotate keys")},
	}
	m.columns[1].issues = []jira.Issue{{Key: "WEB-22", Fields: IssueFields("LOGIN metrics")}}
	m.colCur, m.rowCur = 1, 4

	m, cmd := updateModel(t, m, textKey("/"))
	if cmd == nil || !m.filterActive || m.colCur != 0 || m.rowCur != 0 {
		t.Fatalf("filter did not open with a bounded cursor: active=%v cursor=(%d,%d)", m.filterActive, m.colCur, m.rowCur)
	}
	m.filterInput.SetValue("LoGiN")
	cols := m.visibleColumns()
	if len(cols) != 2 || len(cols[0].issues) != 1 || cols[0].issues[0].Key != "ARA-101" || len(cols[1].issues) != 1 {
		t.Fatalf("summary filter produced unexpected columns: %#v", cols)
	}
	m.filterInput.SetValue("ara-10")
	cols = m.visibleColumns()
	if len(cols[0].issues) != 1 || cols[0].issues[0].Key != "ARA-101" || len(cols[1].issues) != 0 {
		t.Fatalf("key filter produced unexpected columns: %#v", cols)
	}

	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.filterActive || m.filterInput.Value() != "" || len(m.visibleColumns()[0].issues) != 2 {
		t.Fatalf("Escape did not clear local filter: active=%v value=%q", m.filterActive, m.filterInput.Value())
	}
}

func TestLocalFilterEnterSearchesJiraAndRoundTripsToFilteredBoard(t *testing.T) {
	m := makeTestModel(1, 2)
	m.loadedBoardRequest = 7
	m, _ = updateModel(t, m, textKey("/"))
	m.filterInput.SetValue("login")

	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil || m.view != viewSearch || !m.searchLoading || m.searchEditing || m.activeSearchRequest == 0 {
		t.Fatalf("nonempty filter did not start server search: view=%v loading=%v editing=%v request=%d", m.view, m.searchLoading, m.searchEditing, m.activeSearchRequest)
	}
	if got := m.searchInput.Value(); got != "login" {
		t.Fatalf("server query=%q, want local filter value", got)
	}
	requestID := m.activeSearchRequest
	results := []jira.Issue{{Key: "ARA-1", Fields: IssueFields("Login result")}}
	m, _ = updateModel(t, m, searchMsg{requestID: requestID, issues: results, total: 1})
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewKanban || m.filterInput.Value() != "login" {
		t.Fatalf("search did not return to filtered board: view=%v filter=%q", m.view, m.filterInput.Value())
	}

	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewKanban || m.filterInput.Value() != "" {
		t.Fatalf("Escape should clear a retained filter before leaving board: view=%v filter=%q", m.view, m.filterInput.Value())
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewBoards {
		t.Fatalf("second Escape should leave an unfiltered board, got view %v", m.view)
	}
}

func TestEmptyLocalFilterDoesNotStartServerSearch(t *testing.T) {
	m := makeTestModel(1, 1)
	m, _ = updateModel(t, m, textKey("/"))
	m.filterInput.SetValue("  ")
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd != nil || m.view != viewKanban || !m.filterActive || m.activeSearchRequest != 0 {
		t.Fatalf("empty filter unexpectedly searched: cmd=%v view=%v active=%v request=%d", cmd != nil, m.view, m.filterActive, m.activeSearchRequest)
	}
}

func TestDirectSearchReturnsToOpeningView(t *testing.T) {
	tests := []struct {
		name string
		view view
	}{
		{name: "boards", view: viewBoards},
		{name: "kanban", view: viewKanban},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := makeTestModel(1, 1)
			m.view = tt.view
			m, cmd := updateModel(t, m, textKey("s"))
			if cmd == nil || m.view != viewSearch || !m.searchEditing || m.searchReturn != tt.view {
				t.Fatalf("direct search did not open from %v: view=%v editing=%v return=%v", tt.view, m.view, m.searchEditing, m.searchReturn)
			}
			m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
			if m.view != tt.view {
				t.Fatalf("cancelled search returned to %v, want %v", m.view, tt.view)
			}
		})
	}
}

func TestSearchRequestsIgnoreStaleSuccessAndFailure(t *testing.T) {
	m := makeTestModel(1, 1)
	m, _ = startSearchModel(t, m, "first")
	first := m.activeSearchRequest
	m, _ = startSearchModel(t, m, "second")
	second := m.activeSearchRequest
	if second <= first {
		t.Fatalf("second request %d did not supersede first %d", second, first)
	}

	m, _ = updateModel(t, m, searchMsg{
		requestID: first,
		issues:    []jira.Issue{{Key: "OLD-1", Fields: IssueFields("Old")}},
		total:     1,
	})
	m, _ = updateModel(t, m, searchMsg{requestID: first, err: errors.New("stale failure")})
	if !m.searchLoading || m.searchCompleted || m.searchErr != nil || len(m.searchResults) != 0 {
		t.Fatalf("stale search response altered active request: loading=%v completed=%v err=%v results=%d", m.searchLoading, m.searchCompleted, m.searchErr, len(m.searchResults))
	}

	fresh := []jira.Issue{{Key: "NEW-1", Fields: IssueFields("New")}, {Key: "NEW-2", Fields: IssueFields("Newer")}}
	m, _ = updateModel(t, m, searchMsg{requestID: second, issues: fresh, total: 8})
	if m.searchLoading || !m.searchCompleted || m.searchErr != nil || len(m.searchResults) != 2 || m.searchTotal != 8 || m.searchCur != 0 {
		t.Fatalf("active search response was not applied: %#v", m)
	}
}

func TestSearchErrorEmptyAndResizeRendering(t *testing.T) {
	tests := []struct {
		name string
		msg  searchMsg
		want string
	}{
		{name: "error", msg: searchMsg{err: errors.New("search unavailable")}, want: "Error: search unavailable"},
		{name: "empty", msg: searchMsg{}, want: "No tickets found."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := makeTestModel(1, 1)
			m.width, m.height = 34, 8
			m, _ = startSearchModel(t, m, "nothing")
			tt.msg.requestID = m.activeSearchRequest
			m, _ = updateModel(t, m, tt.msg)
			output := m.View().Content
			if !strings.Contains(ansi.Strip(output), tt.want) {
				t.Fatalf("search state missing %q:\n%s", tt.want, output)
			}
			if lipgloss.Width(output) > m.width || lipgloss.Height(output) > m.height {
				t.Fatalf("search output exceeds %dx%d: got %dx%d", m.width, m.height, lipgloss.Width(output), lipgloss.Height(output))
			}
			m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 24, Height: 5})
			output = m.View().Content
			if lipgloss.Width(output) > 24 || lipgloss.Height(output) > 5 {
				t.Fatalf("resized search output exceeds 24x5: got %dx%d", lipgloss.Width(output), lipgloss.Height(output))
			}
		})
	}
}

func TestSearchResultNavigationClampsAndNewSearchCancelKeepsResults(t *testing.T) {
	m := makeTestModel(1, 1)
	m, _ = startSearchModel(t, m, "current")
	requestID := m.activeSearchRequest
	results := []jira.Issue{
		{Key: "A-1", Fields: IssueFields("One")},
		{Key: "A-2", Fields: IssueFields("Two")},
		{Key: "A-3", Fields: IssueFields("Three")},
	}
	m, _ = updateModel(t, m, searchMsg{requestID: requestID, issues: results, total: 3})
	for i := 0; i < 10; i++ {
		m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	}
	if m.searchCur != 2 {
		t.Fatalf("search cursor=%d, want last result", m.searchCur)
	}
	for i := 0; i < 10; i++ {
		m, _ = updateModel(t, m, specialKey(tea.KeyUp))
	}
	if m.searchCur != 0 {
		t.Fatalf("search cursor=%d, want first result", m.searchCur)
	}

	m, _ = updateModel(t, m, textKey("s"))
	if !m.searchEditing {
		t.Fatal("s did not reopen search input")
	}
	m.searchInput.SetValue("replacement")
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.searchEditing || len(m.searchResults) != 3 || m.searchInput.Value() != "current" {
		t.Fatalf("cancelling edited query did not restore result context: editing=%v results=%d query=%q", m.searchEditing, len(m.searchResults), m.searchInput.Value())
	}
}

func TestSearchResultDetailLoadsFullIssueAndReturnsToSameList(t *testing.T) {
	var requested bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		if !strings.Contains(r.URL.Query().Get("fields"), "description") {
			t.Errorf("detail request fields=%q, want full issue fields", r.URL.Query().Get("fields"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"key":"A-2","fields":{"summary":"Full result","description":{"type":"doc","version":1,"content":[]}}}`)
	}))
	defer srv.Close()

	m := makeTestModel(1, 1)
	m.client = jira.NewClient(srv.URL, "test@example.com", "token")
	m, _ = startSearchModel(t, m, "result")
	searchRequest := m.activeSearchRequest
	results := []jira.Issue{{Key: "A-1", Fields: IssueFields("One")}, {Key: "A-2", Fields: IssueFields("Two")}}
	m, _ = updateModel(t, m, searchMsg{requestID: searchRequest, issues: results, total: 2})
	m.searchCur = 1

	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil || m.view != viewDetail || !m.detailLoading || m.detailReturn != viewSearch || m.detailSearchRequest != searchRequest {
		t.Fatalf("result detail did not open: view=%v loading=%v return=%v searchRequest=%d", m.view, m.detailLoading, m.detailReturn, m.detailSearchRequest)
	}
	commandResult := cmd()
	detailResult, ok := commandResult.(detailMsg)
	if !ok {
		t.Fatalf("detail command returned %T, want detailMsg", commandResult)
	}
	if !requested || detailResult.issue == nil || detailResult.issue.Fields.Description == nil {
		t.Fatalf("search detail did not load full issue: requested=%v result=%#v", requested, detailResult)
	}

	m, _ = updateModel(t, m, detailMsg{
		issue:         detailResult.issue,
		issueKey:      detailResult.issueKey,
		searchRequest: searchRequest + 1,
		detailRequest: m.activeDetailRequest,
	})
	if !m.detailLoading || m.detail != nil {
		t.Fatal("detail response from a different search request was accepted")
	}
	m, _ = updateModel(t, m, detailResult)
	if m.detailLoading || m.detail == nil || m.detail.Key != "A-2" {
		t.Fatalf("matching full detail was not displayed: loading=%v detail=%v", m.detailLoading, m.detail)
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewSearch || len(m.searchResults) != 2 || m.searchCur != 1 || m.searchInput.Value() != "result" {
		t.Fatalf("closing detail lost result context: view=%v results=%d cursor=%d query=%q", m.view, len(m.searchResults), m.searchCur, m.searchInput.Value())
	}
}

func TestSearchAndSprintActionsUseDistinctKeys(t *testing.T) {
	m := makeTestModel(1, 1)
	m.sprintID = 42
	m.sprintName = "Active Sprint"
	m.loadedBoardRequest = 3

	searched, searchCmd := updateModel(t, m, textKey("s"))
	if searchCmd == nil || searched.view != viewSearch || !searched.searchEditing {
		t.Fatalf("s did not open direct search: view=%v editing=%v", searched.view, searched.searchEditing)
	}
	added, addCmd := updateModel(t, m, textKey("a"))
	if addCmd == nil || added.view != viewKanban {
		t.Fatalf("a did not start active-sprint action: cmd=%v view=%v", addCmd != nil, added.view)
	}

	m.sprintID = 0
	m, cmd := updateModel(t, m, textKey("a"))
	if cmd == nil || m.toast != "No active sprint on this board" {
		t.Fatalf("missing-sprint action did not provide feedback: cmd=%v toast=%q", cmd != nil, m.toast)
	}
}

func TestSprintRefreshCompletesWhileSearchIsOpen(t *testing.T) {
	m := makeTestModel(1, 1)
	m.sprintID = 42
	m.sprintName = "Active Sprint"
	m.loadedBoardRequest = 3

	m, addCmd := updateModel(t, m, textKey("a"))
	if addCmd == nil {
		t.Fatal("a did not start active-sprint action")
	}
	m, _ = updateModel(t, m, textKey("s"))
	if m.view != viewSearch || !m.searchEditing {
		t.Fatalf("s did not open search while sprint action was pending: view=%v editing=%v", m.view, m.searchEditing)
	}

	m, refreshCmd := updateModel(t, m, sprintDoneMsg{issueKey: "TEST-1"})
	if refreshCmd == nil || !m.boardLoading || m.activeBoardRequest == 0 {
		t.Fatalf("sprint completion did not start board refresh: cmd=%v loading=%v request=%d", refreshCmd != nil, m.boardLoading, m.activeBoardRequest)
	}
	requestID := m.activeBoardRequest
	m, _ = updateModel(t, m, boardResult(requestID, 1, "Refreshed Sprint", "TEST-1"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))

	if m.view != viewKanban || m.boardLoading || m.loadedBoardRequest != requestID {
		t.Fatalf("return from search left board refresh stuck: view=%v loading=%v loaded=%d want=%d", m.view, m.boardLoading, m.loadedBoardRequest, requestID)
	}
	if len(m.columns) != 1 || len(m.columns[0].issues) != 1 || m.columns[0].issues[0].Key != "TEST-1" {
		t.Fatalf("refreshed board was not available after search: %#v", m.columns)
	}
}

func TestSprintRefreshSupersedesOpenTransitionPicker(t *testing.T) {
	m := makeTestModel(1, 1)
	m.sprintID = 42
	m.sprintName = "Active Sprint"
	m.loadedBoardRequest = 3

	m, addCmd := updateModel(t, m, textKey("a"))
	if addCmd == nil {
		t.Fatal("a did not start active-sprint action")
	}
	m, transitionCmd := updateModel(t, m, textKey("t"))
	if transitionCmd == nil || !m.transitionLoading {
		t.Fatalf("t did not start transition fetch: cmd=%v loading=%v", transitionCmd != nil, m.transitionLoading)
	}
	transitionRequest := m.activeTransitionRequest
	m, _ = updateModel(t, m, transitionsMsg{
		requestID:    transitionRequest,
		transitions:  []jira.Transition{{ID: "2", Name: "Done"}},
		issueKey:     "P0",
		boardRequest: 3,
	})
	if m.view != viewTransition || len(m.transitions) != 1 {
		t.Fatalf("transition picker did not open: view=%v transitions=%d", m.view, len(m.transitions))
	}

	m, refreshCmd := updateModel(t, m, sprintDoneMsg{issueKey: "P0"})
	if refreshCmd == nil || m.view != viewKanban || !m.boardLoading || m.activeBoardRequest == 0 {
		t.Fatalf("sprint refresh did not return picker to loading board: cmd=%v view=%v loading=%v request=%d", refreshCmd != nil, m.view, m.boardLoading, m.activeBoardRequest)
	}
	if m.transitionLoading || m.activeTransitionRequest != 0 || len(m.transitions) != 0 || m.transIssue != "" {
		t.Fatalf("sprint refresh retained obsolete transition picker: loading=%v request=%d transitions=%d issue=%q", m.transitionLoading, m.activeTransitionRequest, len(m.transitions), m.transIssue)
	}
	if m.toast != "Added P0 to Active Sprint" || m.focusIssue != "P0" {
		t.Fatalf("sprint completion was hidden: toast=%q focus=%q", m.toast, m.focusIssue)
	}
}

func TestSprintRefreshSupersedesInFlightBoardDetail(t *testing.T) {
	m := makeTestModel(1, 1)
	m.sprintID = 42
	m.sprintName = "Active Sprint"
	m.loadedBoardRequest = 3

	m, addCmd := updateModel(t, m, textKey("a"))
	if addCmd == nil {
		t.Fatal("a did not start active-sprint action")
	}
	m, detailCmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if detailCmd == nil || m.view != viewDetail || !m.detailLoading || m.detailReturn != viewKanban {
		t.Fatalf("board detail did not open while sprint action was pending: view=%v loading=%v return=%v", m.view, m.detailLoading, m.detailReturn)
	}
	oldDetailRequest := m.activeDetailRequest

	m, refreshCmd := updateModel(t, m, sprintDoneMsg{issueKey: "P0"})
	if refreshCmd == nil || m.view != viewKanban || !m.boardLoading || m.detailLoading || m.activeDetailRequest != 0 {
		t.Fatalf("board refresh did not supersede board detail safely: cmd=%v view=%v boardLoading=%v detailLoading=%v detailRequest=%d", refreshCmd != nil, m.view, m.boardLoading, m.detailLoading, m.activeDetailRequest)
	}
	boardRequest := m.activeBoardRequest
	m, _ = updateModel(t, m, boardResult(boardRequest, 1, "Refreshed Sprint", "P0"))

	staleIssue := testIssue("P0")
	m, _ = updateModel(t, m, detailMsg{
		issue:         &staleIssue,
		issueKey:      staleIssue.Key,
		boardRequest:  3,
		detailRequest: oldDetailRequest,
	})
	if m.view != viewKanban || m.boardLoading || m.detailLoading || m.detail != nil || m.loadedBoardRequest != boardRequest {
		t.Fatalf("stale detail disturbed refreshed board: view=%v boardLoading=%v detailLoading=%v detail=%v loaded=%d want=%d", m.view, m.boardLoading, m.detailLoading, m.detail, m.loadedBoardRequest, boardRequest)
	}
}

func TestSprintRefreshPreservesInFlightSearchDetail(t *testing.T) {
	m := makeTestModel(1, 1)
	m.sprintID = 42
	m.sprintName = "Active Sprint"
	m.loadedBoardRequest = 3

	m, _ = updateModel(t, m, textKey("a"))
	m, _ = updateModel(t, m, textKey("s"))
	m.searchInput.SetValue("result")
	m, searchCmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if searchCmd == nil || !m.searchLoading {
		t.Fatal("search did not start while sprint action was pending")
	}
	searchRequest := m.activeSearchRequest

	m, _ = updateModel(t, m, sprintDoneMsg{issueKey: "TEST-1"})
	boardRequest := m.activeBoardRequest
	if m.activeSearchRequest != searchRequest {
		t.Fatalf("board refresh invalidated active search: got=%d want=%d", m.activeSearchRequest, searchRequest)
	}
	results := []jira.Issue{{Key: "RESULT-1", Fields: IssueFields("Search result")}}
	m, _ = updateModel(t, m, searchMsg{requestID: searchRequest, issues: results, total: 1})
	m, detailCmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if detailCmd == nil || m.view != viewDetail || !m.detailLoading {
		t.Fatalf("search detail did not open during board refresh: view=%v loading=%v", m.view, m.detailLoading)
	}
	detailRequest := m.activeDetailRequest

	m, _ = updateModel(t, m, boardResult(boardRequest, 1, "Refreshed Sprint", "TEST-1"))
	issue := testIssue("RESULT-1")
	m, _ = updateModel(t, m, detailMsg{
		issue:         &issue,
		issueKey:      issue.Key,
		searchRequest: searchRequest,
		detailRequest: detailRequest,
	})
	if m.detailLoading || m.detail == nil || m.detail.Key != "RESULT-1" {
		t.Fatalf("board refresh invalidated search detail: loading=%v detail=%v", m.detailLoading, m.detail)
	}

	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewKanban || m.boardLoading || m.loadedBoardRequest != boardRequest {
		t.Fatalf("search detail returned to stale board: view=%v loading=%v loaded=%d want=%d", m.view, m.boardLoading, m.loadedBoardRequest, boardRequest)
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
		{
			name: "local filter",
			model: func() Model {
				input := textinput.New()
				input.Focus()
				return Model{view: viewKanban, filterActive: true, filterInput: input}
			}(),
			wantValue: func(m Model) string { return m.filterInput.Value() },
		},
		{
			name: "server search",
			model: func() Model {
				input := textinput.New()
				input.Focus()
				return Model{view: viewSearch, searchEditing: true, searchInput: input}
			}(),
			wantValue: func(m Model) string { return m.searchInput.Value() },
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

func TestKanbanStatusRemainsContextualAcrossWidthsResizeAndToast(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		toast  string
		wants  []string
	}{
		{
			name:   "wide",
			width:  190,
			height: 32,
			wants:  []string{"Board: Test Board", "Sprint: Sprint 24", "Column 2/5", "Page 1/1", "[←→] Cols"},
		},
		{
			name:   "compact footer",
			width:  60,
			height: 14,
			wants:  []string{"Board: Test Board", "Sprint: Sprint 24", "Col 2/5", "Page 1/3", "[?] Help", "[esc] Back"},
		},
		{
			name:   "compact with toast",
			width:  60,
			height: 14,
			toast:  "Board updated",
			wants:  []string{"Board: Test Board", "Sprint: Sprint 24", "Col 2/5", "Page 1/3", "Board updated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := makeTestModel(5, 2)
			m.width, m.height = tt.width, tt.height
			m.colCur = 1
			m.sprintName = "Sprint 24"
			m.toast = tt.toast
			output := ansi.Strip(m.View().Content)
			for _, want := range tt.wants {
				if !strings.Contains(output, want) {
					t.Fatalf("status missing %q:\n%s", want, output)
				}
			}
			if lipgloss.Width(m.View().Content) > tt.width || lipgloss.Height(m.View().Content) > tt.height {
				t.Fatalf("status view exceeds %dx%d", tt.width, tt.height)
			}
		})
	}

	m := makeTestModel(5, 2)
	m.sprintName = "Sprint 24"
	m.colCur = 4
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 60, Height: 14})
	output := ansi.Strip(m.View().Content)
	for _, want := range []string{"Board: Test Board", "Sprint: Sprint 24", "Col 5/5", "Page 3/3"} {
		if !strings.Contains(output, want) {
			t.Fatalf("resized status missing %q:\n%s", want, output)
		}
	}
}

func TestKanbanCompactStatusPreservesLongBoardAndSprintContext(t *testing.T) {
	m := makeTestModel(5, 2)
	m.width, m.height = 60, 14
	m.boards[0].Name = strings.Repeat("Board", 10)
	m.sprintName = "Sprint 24"
	m.colCur = 1

	output := ansi.Strip(m.View().Content)
	for _, want := range []string{"Board: BoardBoardBoard", "Sprint: Sprint 24", "Col 2/5", "Page 1/3"} {
		if !strings.Contains(output, want) {
			t.Fatalf("compact status missing %q:\n%s", want, output)
		}
	}
}

func TestKanbanLoadingStatusUsesTargetBoardWithoutStaleSprint(t *testing.T) {
	m := makeTestModel(1, 1)
	m.view = viewBoards
	m.boards = []jira.Board{{ID: 1, Name: "Old Board"}, {ID: 2, Name: "Target Board"}}
	m.boardCur = 1
	m.sprintName = "Old Sprint"

	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	output := ansi.Strip(m.View().Content)
	for _, want := range []string{"Board: Target Board", "Sprint: Loading", "Loading Target Board..."} {
		if !strings.Contains(output, want) {
			t.Fatalf("loading status missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "Old Sprint") || strings.Contains(output, "Old Board") {
		t.Fatalf("loading status retained stale context:\n%s", output)
	}
}

func TestTransitionFetchFeedbackDeduplicatesAndScopesReplies(t *testing.T) {
	m := makeTestModel(1, 1)
	m.loadedBoardRequest = 7

	m, firstCmd := updateModel(t, m, textKey("t"))
	firstRequest := m.activeTransitionRequest
	if firstCmd == nil || !m.transitionLoading || firstRequest == 0 {
		t.Fatalf("transition fetch did not start: cmd=%v loading=%v request=%d", firstCmd != nil, m.transitionLoading, firstRequest)
	}
	if output := ansi.Strip(m.View().Content); !strings.Contains(output, "Loading transitions for P0") {
		t.Fatalf("fetch feedback is not visible:\n%s", output)
	}
	m, duplicateCmd := updateModel(t, m, textKey("t"))
	if duplicateCmd != nil || m.activeTransitionRequest != firstRequest {
		t.Fatalf("duplicate fetch started: cmd=%v request=%d", duplicateCmd != nil, m.activeTransitionRequest)
	}

	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewKanban || m.transitionLoading || m.activeTransitionRequest != 0 {
		t.Fatalf("fetch cancel did not clear activity: view=%v loading=%v request=%d", m.view, m.transitionLoading, m.activeTransitionRequest)
	}
	m, _ = updateModel(t, m, textKey("t"))
	secondRequest := m.activeTransitionRequest
	m, _ = updateModel(t, m, transitionsMsg{
		requestID:    firstRequest,
		transitions:  []jira.Transition{{ID: "2", Name: "Done"}},
		issueKey:     "P0",
		boardRequest: 7,
	})
	if !m.transitionLoading || m.activeTransitionRequest != secondRequest || m.view != viewKanban {
		t.Fatal("late cancelled fetch replaced the active request")
	}
	m, _ = updateModel(t, m, transitionsMsg{
		requestID:    secondRequest,
		transitions:  []jira.Transition{{ID: "2", Name: "Done"}},
		issueKey:     "P0",
		boardRequest: 7,
	})
	if m.transitionLoading || m.activeTransitionRequest != 0 || m.view != viewTransition {
		t.Fatalf("matching fetch did not open picker: loading=%v request=%d view=%v", m.transitionLoading, m.activeTransitionRequest, m.view)
	}
}

func TestTransitionFetchErrorClearsFeedback(t *testing.T) {
	m := makeTestModel(1, 1)
	m.loadedBoardRequest = 7
	m, _ = updateModel(t, m, textKey("t"))
	requestID := m.activeTransitionRequest
	fetchErr := errors.New("transitions unavailable")
	m, _ = updateModel(t, m, transitionsMsg{requestID: requestID, issueKey: "P0", boardRequest: 7, err: fetchErr})
	if m.transitionLoading || m.activeTransitionRequest != 0 || !errors.Is(m.err, fetchErr) || m.view != viewKanban {
		t.Fatalf("fetch error state not cleared: loading=%v request=%d err=%v view=%v", m.transitionLoading, m.activeTransitionRequest, m.err, m.view)
	}
}

func TestTransitionSubmitFeedbackDeduplicatesCancelsAndIgnoresLateReplies(t *testing.T) {
	m := makeTestModel(1, 1)
	m.loadedBoardRequest = 7
	m.view = viewTransition
	m.transitions = []jira.Transition{{ID: "2", Name: "Done"}}
	m.transIssue = "P0"

	m, firstCmd := updateModel(t, m, specialKey(tea.KeyEnter))
	firstRequest := m.activeTransitionSubmitRequest
	if firstCmd == nil || !m.transitionSubmitting || firstRequest == 0 {
		t.Fatalf("transition submit did not start: cmd=%v submitting=%v request=%d", firstCmd != nil, m.transitionSubmitting, firstRequest)
	}
	if output := ansi.Strip(m.View().Content); !strings.Contains(output, "Moving P0 to Done...") {
		t.Fatalf("submit feedback is not visible:\n%s", output)
	}
	m.width, m.height = 24, 6
	bounded := m.View().Content
	if lipgloss.Width(bounded) > m.width || lipgloss.Height(bounded) > m.height || !strings.Contains(ansi.Strip(bounded), "Moving P0") {
		t.Fatalf("submit feedback exceeds compact bounds or became invisible:\n%s", bounded)
	}
	m, duplicateCmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if duplicateCmd != nil || m.activeTransitionSubmitRequest != firstRequest {
		t.Fatalf("double Enter started a duplicate submit: cmd=%v request=%d", duplicateCmd != nil, m.activeTransitionSubmitRequest)
	}

	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewKanban || m.transitionSubmitting || m.activeTransitionSubmitRequest != 0 {
		t.Fatalf("submit cancel did not clear activity: view=%v submitting=%v request=%d", m.view, m.transitionSubmitting, m.activeTransitionSubmitRequest)
	}
	m, _ = updateModel(t, m, transitionDoneMsg{requestID: firstRequest, boardRequest: 7, issueKey: "P0"})
	if m.view != viewKanban || m.toast != "" || m.boardLoading {
		t.Fatal("late cancelled submit changed the board")
	}
}

func TestTransitionSubmitCompactShowsActivityAndCancelWithLongList(t *testing.T) {
	m := makeTestModel(1, 1)
	m.width, m.height = 60, 6
	m.loadedBoardRequest = 7
	m.view = viewTransition
	m.transitions = []jira.Transition{
		{ID: "2", Name: "In Progress"},
		{ID: "3", Name: "Review"},
		{ID: "4", Name: "Done"},
	}
	m.transCur = 2
	m.transIssue = "P0"

	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	output := ansi.Strip(m.View().Content)
	for _, want := range []string{"Moving P0 to Done...", "[esc] Cancel"} {
		if !strings.Contains(output, want) {
			t.Fatalf("compact submit missing %q:\n%s", want, output)
		}
	}
}

func TestTransitionSubmitSuccessAndErrorClearActivity(t *testing.T) {
	for _, tt := range []struct {
		name    string
		err     error
		wantErr bool
	}{
		{name: "success"},
		{name: "error", err: errors.New("transition failed"), wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := makeTestModel(1, 1)
			m.loadedBoardRequest = 7
			m.view = viewTransition
			m.transitions = []jira.Transition{{ID: "2", Name: "Done"}}
			m.transIssue = "P0"
			m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
			requestID := m.activeTransitionSubmitRequest
			m, cmd := updateModel(t, m, transitionDoneMsg{requestID: requestID, boardRequest: 7, issueKey: "P0", err: tt.err})
			if m.transitionSubmitting || m.activeTransitionSubmitRequest != 0 || m.view != viewKanban {
				t.Fatalf("completion did not clear submit state: submitting=%v request=%d view=%v", m.transitionSubmitting, m.activeTransitionSubmitRequest, m.view)
			}
			if tt.wantErr {
				if cmd != nil || !errors.Is(m.err, tt.err) || m.boardLoading {
					t.Fatalf("submit error changed behavior: cmd=%v err=%v loading=%v", cmd != nil, m.err, m.boardLoading)
				}
			} else if cmd == nil || m.toast != "Moved P0" || !m.boardLoading {
				t.Fatalf("submit success did not refresh board: cmd=%v toast=%q loading=%v", cmd != nil, m.toast, m.boardLoading)
			}
		})
	}
}

func TestTransitionSubmitCompletionSurvivesSameBoardSprintRefresh(t *testing.T) {
	transitionErr := errors.New("transition failed after sprint refresh")
	for _, tt := range []struct {
		name                string
		transitionErr       error
		completeBoardBefore bool
	}{
		{name: "success before refreshed board"},
		{name: "success after refreshed board", completeBoardBefore: true},
		{name: "error before refreshed board", transitionErr: transitionErr},
		{name: "error after refreshed board", transitionErr: transitionErr, completeBoardBefore: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := makeTestModel(1, 1)
			m.sprintID = 42
			m.sprintName = "Active Sprint"
			m.loadedBoardRequest = 7

			m, addCmd := updateModel(t, m, textKey("a"))
			if addCmd == nil {
				t.Fatal("a did not start active-sprint action")
			}
			m, transitionCmd := updateModel(t, m, textKey("t"))
			if transitionCmd == nil {
				t.Fatal("t did not start transition fetch")
			}
			m, _ = updateModel(t, m, transitionsMsg{
				requestID:    m.activeTransitionRequest,
				transitions:  []jira.Transition{{ID: "2", Name: "Done"}},
				issueKey:     "P0",
				boardRequest: 7,
			})
			m, submitCmd := updateModel(t, m, specialKey(tea.KeyEnter))
			if submitCmd == nil || !m.transitionSubmitting {
				t.Fatal("Enter did not start transition mutation")
			}
			submitRequest := m.activeTransitionSubmitRequest

			m, sprintRefreshCmd := updateModel(t, m, sprintDoneMsg{issueKey: "P0"})
			if sprintRefreshCmd == nil || m.view != viewKanban || !m.boardLoading {
				t.Fatalf("sprint completion did not dismiss picker and refresh board: cmd=%v view=%v loading=%v", sprintRefreshCmd != nil, m.view, m.boardLoading)
			}
			if !m.transitionSubmitting || m.activeTransitionSubmitRequest != submitRequest {
				t.Fatalf("same-board refresh lost transition mutation identity: submitting=%v request=%d want=%d", m.transitionSubmitting, m.activeTransitionSubmitRequest, submitRequest)
			}
			sprintBoardRequest := m.activeBoardRequest
			if tt.completeBoardBefore {
				m, _ = updateModel(t, m, boardResult(sprintBoardRequest, 1, "Refreshed Sprint", "P0"))
			}

			m, transitionResultCmd := updateModel(t, m, transitionDoneMsg{
				requestID:    submitRequest,
				boardRequest: 7,
				issueKey:     "P0",
				err:          tt.transitionErr,
			})
			if m.transitionSubmitting || m.activeTransitionSubmitRequest != 0 || m.view != viewKanban || len(m.transitions) != 0 || m.transIssue != "" {
				t.Fatalf("completion revived or retained picker state: submitting=%v request=%d view=%v transitions=%d issue=%q", m.transitionSubmitting, m.activeTransitionSubmitRequest, m.view, len(m.transitions), m.transIssue)
			}

			if tt.transitionErr == nil {
				if transitionResultCmd == nil || m.toast != "Moved P0" || !m.boardLoading || m.activeBoardRequest == sprintBoardRequest {
					t.Fatalf("success did not start a fresh scoped refresh: cmd=%v toast=%q loading=%v request=%d prior=%d", transitionResultCmd != nil, m.toast, m.boardLoading, m.activeBoardRequest, sprintBoardRequest)
				}
			} else {
				if transitionResultCmd != nil || !errors.Is(m.err, tt.transitionErr) {
					t.Fatalf("error was not surfaced: cmd=%v err=%v", transitionResultCmd != nil, m.err)
				}
				if !tt.completeBoardBefore {
					m, _ = updateModel(t, m, boardResult(sprintBoardRequest, 1, "Refreshed Sprint", "P0"))
					if !errors.Is(m.err, tt.transitionErr) || m.boardLoading {
						t.Fatalf("board completion hid transition error: err=%v loading=%v", m.err, m.boardLoading)
					}
				}
			}
		})
	}
}

func TestTransitionSubmitCompletionIsRejectedAfterExplicitBoardSwitch(t *testing.T) {
	m := makeTestModel(1, 1)
	m.boards = append(m.boards, jira.Board{ID: 2, Name: "Other Board", Type: "scrum"})
	m.sprintID = 42
	m.sprintName = "Active Sprint"
	m.loadedBoardRequest = 7
	m.view = viewTransition
	m.transitions = []jira.Transition{{ID: "2", Name: "Done"}}
	m.transIssue = "P0"

	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	submitRequest := m.activeTransitionSubmitRequest
	m, _ = updateModel(t, m, sprintDoneMsg{issueKey: "P0"})
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	otherBoardRequest := m.activeBoardRequest
	if m.loadingBoardID != 2 {
		t.Fatalf("explicit board switch did not start Other Board load: board=%d", m.loadingBoardID)
	}

	m, cmd := updateModel(t, m, transitionDoneMsg{requestID: submitRequest, boardRequest: 7, issueKey: "P0"})
	if cmd != nil || m.toast == "Moved P0" || m.activeBoardRequest != otherBoardRequest || m.loadingBoardID != 2 {
		t.Fatalf("old-board mutation response affected new board: cmd=%v toast=%q request=%d board=%d", cmd != nil, m.toast, m.activeBoardRequest, m.loadingBoardID)
	}
}
