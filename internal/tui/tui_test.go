package tui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
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
				return detailMsg{issue: &issue, boardRequest: boardRequest}
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
		response func(uint64) tea.Msg
		wantView view
	}{
		{
			name: "detail",
			response: func(boardRequest uint64) tea.Msg {
				issue := testIssue("A-1")
				return detailMsg{issue: &issue, boardRequest: boardRequest}
			},
			wantView: viewDetail,
		},
		{
			name: "transitions",
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
			m, _ = updateModel(t, m, tt.response(m.loadedBoardRequest))
			if m.view != tt.wantView {
				t.Fatalf("current-board response opened view %v, want %v", m.view, tt.wantView)
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
