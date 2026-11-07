package tui

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestExportOpensWithVisibleBoardScope(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	view := m.View().Content
	if !strings.Contains(view, "Export") || !strings.Contains(view, "Delivery") || !strings.Contains(view, "42") {
		t.Fatalf("export scope not visible: %q", view)
	}
}

func TestExportShowsAllBoardIssuesAsInitialScope(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	if !strings.Contains(m.View().Content, "All board issues") {
		t.Fatalf("initial export scope is not visible: %q", m.View().Content)
	}
}

func TestExportCanRequestSprintSelection(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("s"))
	if !strings.Contains(m.View().Content, "Sprint ID") {
		t.Fatalf("sprint selection not shown: %q", m.View().Content)
	}
}

func TestExportSelectsSprintByID(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, textKey("7"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	if !strings.Contains(m.View().Content, "Sprint: 7") {
		t.Fatalf("selected sprint not visible: %q", m.View().Content)
	}
}

func TestExportFetchesSelectedBoardAndSprint(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	var board, sprint int
	m.exportFetch = func(b, s int) ([]jira.Issue, error) { board, sprint = b, s; return nil, nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, textKey("7"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	_, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("export did not request issues")
	}
	cmd()
	if board != 42 || sprint != 7 {
		t.Fatalf("fetch scope = %d/%d, want 42/7", board, sprint)
	}
}

func TestExportReopensWithAllBoardIssues(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, textKey("7"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, textKey("E"))
	if !strings.Contains(m.View().Content, "All board issues") {
		t.Fatalf("stale scope: %q", m.View().Content)
	}
}

func TestExportWarnsCompletenessUnknownBeforeCopy(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	view := m.View().Content
	if !strings.Contains(view, "completeness unknown") || !strings.Contains(view, "2000") {
		t.Fatalf("missing completeness warning: %q", view)
	}
}

func TestExportCopiesCuratedTickets(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return []jira.Issue{{Key: "ABC-1"}}, nil }
	var copied string
	m.exportCopy = func(s string) error { copied = s; return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, cmd = updateModel(t, m, cmd())
	if cmd == nil {
		t.Fatal("successful fetch did not request clipboard copy")
	}
	cmd()
	var got []jira.TicketJSON
	if err := json.Unmarshal([]byte(copied), &got); err != nil || len(got) != 1 || got[0].Key != "ABC-1" || got[0].SchemaVersion == "" {
		t.Fatalf("clipboard is not curated TicketJSON: %q (%v)", copied, err)
	}
}

func TestExportIgnoresFetchFromPreviousView(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return []jira.Issue{{Key: "OLD-1"}}, nil }
	m.exportCopy = func(string) error { t.Fatal("stale result copied"); return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	old := cmd()
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, textKey("E"))
	_, cmd = updateModel(t, m, old)
	if cmd != nil {
		t.Fatal("stale result scheduled a copy")
	}
}

func TestExportIgnoresFetchAfterScopeChange(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	old := cmd()
	m, _ = updateModel(t, m, textKey("s"))
	_, cmd = updateModel(t, m, old)
	if cmd != nil {
		t.Fatal("old scope scheduled a copy")
	}
}

func TestExportFetchFailureIsVisibleAndDoesNotCopy(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, errors.New("fetch failed") }
	m.exportCopy = func(string) error { t.Fatal("copied after fetch failure"); return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, cmd = updateModel(t, m, cmd())
	if cmd != nil || !strings.Contains(m.View().Content, "fetch failed") {
		t.Fatalf("fetch error not visible: %q", m.View().Content)
	}
}

func TestExportClipboardFailureIsVisible(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	m.exportCopy = func(string) error { return errors.New("clipboard unavailable") }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, cmd = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, cmd())
	if !strings.Contains(m.View().Content, "clipboard unavailable") {
		t.Fatalf("clipboard failure not visible: %q", m.View().Content)
	}
}

func TestExportRejectsInvalidSprintID(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, textKey("0"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd != nil || !strings.Contains(m.View().Content, "positive sprint ID") {
		t.Fatalf("invalid sprint not rejected visibly: %q", m.View().Content)
	}
}

func TestExportSprintInputSupportsBackspace(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, textKey("1"))
	m, _ = updateModel(t, m, textKey("2"))
	m, _ = updateModel(t, m, specialKey(tea.KeyBackspace))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	if !strings.Contains(m.View().Content, "Sprint: 1\n") {
		t.Fatalf("backspace did not edit sprint ID: %q", m.View().Content)
	}
}

func TestExportCancelsSprintEntryWithoutLeaving(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewExport || strings.Contains(m.View().Content, "Sprint ID:") {
		t.Fatalf("sprint entry not cancelled: %q", m.View().Content)
	}
}

func TestExportIgnoresClipboardCompletionFromPreviousView(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	m.exportCopy = func(string) error { return errors.New("old clipboard failure") }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, cmd = updateModel(t, m, cmd())
	old := cmd()
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, old)
	if strings.Contains(m.View().Content, "old clipboard failure") {
		t.Fatal("stale clipboard error appeared on board")
	}
}

func TestExportClipboardSuccessIsVisible(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	m.exportCopy = func(string) error { return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, cmd = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, cmd())
	if !strings.Contains(m.View().Content, "Export copied to clipboard") {
		t.Fatalf("success not visible: %q", m.View().Content)
	}
}

func TestExportCancelPreventsPendingClipboardWrite(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	copied := false
	m.exportCopy = func(string) error { copied = true; return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, cmd = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	cmd()
	if copied {
		t.Fatal("cancelled export wrote to clipboard")
	}
}

func TestExportScopeChangePreventsPendingClipboardWrite(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	copied := false
	m.exportCopy = func(string) error { copied = true; return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, cmd = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, textKey("s"))
	cmd()
	if copied {
		t.Fatal("previous scope wrote to clipboard")
	}
}

func TestExportNewRequestPreventsPreviousClipboardWrite(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	copied := false
	m.exportCopy = func(string) error { copied = true; return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, oldCopy := updateModel(t, m, cmd())
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	oldCopy()
	if copied {
		t.Fatal("superseded export wrote to clipboard")
	}
}

func TestExportSuccessfulSprintEntryClearsValidationError(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, textKey("7"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	if strings.Contains(m.View().Content, "positive sprint ID") {
		t.Fatalf("stale validation error: %q", m.View().Content)
	}
}

func TestExportShowsControlsBeforeCopy(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	view := m.View().Content
	if !strings.Contains(view, "[s]") || !strings.Contains(view, "[Enter]") || !strings.Contains(view, "[Esc]") {
		t.Fatalf("export controls missing: %q", view)
	}
}

func TestExportCtrlCQuits(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	_, cmd := updateModel(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not return QuitMsg")
	}
}

func TestExistingCopyAndDetailEditShortcuts(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.columns = []column{{name: "Todo", issues: []jira.Issue{{Key: "ABC-1"}}}}
	_, cmd := updateModel(t, m, textKey("C"))
	if cmd == nil {
		t.Fatal("column copy unavailable")
	}
	m.view = viewDetail
	m.detail = &jira.Issue{Key: "ABC-1"}
	_, cmd = updateModel(t, m, textKey("c"))
	if cmd == nil {
		t.Fatal("detail copy unavailable")
	}
	m, _ = updateModel(t, m, textKey("E"))
	if m.view != viewFieldsEdit {
		t.Fatal("detail E no longer opens field editing")
	}
}

func TestExportBoardNavigationRejectsPreviousBoardResponse(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}, {ID: 99, Name: "Other"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return []jira.Issue{{Key: "OLD-1"}}, nil }
	m.exportCopy = func(string) error { t.Fatal("previous board copied"); return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	old := cmd()
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, boardLoadMsg{requestID: m.activeBoardRequest, boardID: 99})
	m, _ = updateModel(t, m, textKey("E"))
	if !strings.Contains(m.View().Content, "Other") {
		t.Fatalf("wrong board scope: %q", m.View().Content)
	}
	_, cmd = updateModel(t, m, old)
	if cmd != nil {
		t.Fatal("previous board response scheduled a copy")
	}
}

func TestExportAlreadyStartedClipboardWriteCannotBeUndone(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan tea.Msg, 1)
	m.exportCopy = func(string) error { close(started); <-release; return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, cmd = updateModel(t, m, cmd())
	go func() { done <- cmd() }()
	<-started
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	close(release)
	m, _ = updateModel(t, m, <-done)
	if strings.Contains(m.View().Content, "Export copied") {
		t.Fatal("stale success appeared after leaving")
	}
}

func TestExportSprintInputRejectsNondigits(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, textKey("x"))
	if !strings.Contains(m.View().Content, "digits only") {
		t.Fatalf("invalid character silently ignored: %q", m.View().Content)
	}
}

func TestExportRetryClearsPreviousFetchError(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, errors.New("fetch failed") }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	if strings.Contains(m.View().Content, "fetch failed") {
		t.Fatalf("stale fetch error remains during retry: %q", m.View().Content)
	}
}

func TestExportNewRequestClearsPreviousSuccess(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	m.exportCopy = func(string) error { return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, cmd = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	if strings.Contains(m.View().Content, "Export copied") {
		t.Fatalf("stale success during new request: %q", m.View().Content)
	}
}

func TestExportScopeChangeClearsPreviousSuccess(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	m.exportCopy = func(string) error { return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, cmd = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, textKey("s"))
	if strings.Contains(m.View().Content, "Export copied") {
		t.Fatalf("stale success while changing scope: %q", m.View().Content)
	}
}

func TestExportReopenClearsPreviousError(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, errors.New("old fetch error") }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, textKey("E"))
	if strings.Contains(m.View().Content, "old fetch error") {
		t.Fatal("stale error persisted after reopening")
	}
}

func TestExportCanRequestBoardID(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("b"))
	if !strings.Contains(m.View().Content, "Board ID:") {
		t.Fatalf("board entry not visible: %q", m.View().Content)
	}
}

func TestExportSelectsExplicitBoardIDVisibly(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, textKey("9"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	if !strings.Contains(m.View().Content, "Board ID: 9") {
		t.Fatalf("explicit board not visible: %q", m.View().Content)
	}
}

func TestExportRejectsInvalidBoardWithoutFetching(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { t.Fatal("invalid board fetched"); return nil, nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, textKey("0"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd != nil || !strings.Contains(m.View().Content, "positive board ID") {
		t.Fatalf("invalid board not rejected: %q", m.View().Content)
	}
}

func TestExportFetchesExplicitBoardAndSelectedSprint(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	var board, sprint int
	m.exportFetch = func(b, s int) ([]jira.Issue, error) { board, sprint = b, s; return nil, nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, textKey("7"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, textKey("9"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	_, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("no export fetch")
	}
	cmd()
	if board != 9 || sprint != 7 {
		t.Fatalf("scope = %d/%d, want 9/7", board, sprint)
	}
}

func TestExportBoardEntryInvalidatesOldFetch(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	m.exportCopy = func(string) error { t.Fatal("stale scope copied"); return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	old := cmd()
	m, _ = updateModel(t, m, textKey("b"))
	_, cmd = updateModel(t, m, old)
	if cmd != nil {
		t.Fatal("old board fetch scheduled a copy")
	}
}

func TestExportBoardEntryEscapeCancelsEntry(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewExport || strings.Contains(m.View().Content, "Board ID:") {
		t.Fatalf("board entry not cancelled: %q", m.View().Content)
	}
}

func TestExportBoardInputSupportsBackspace(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, textKey("9"))
	m, _ = updateModel(t, m, textKey("8"))
	m, _ = updateModel(t, m, specialKey(tea.KeyBackspace))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	if !strings.Contains(m.View().Content, "Board ID: 9\n") {
		t.Fatalf("backspace did not edit board ID: %q", m.View().Content)
	}
}

func TestExportReopenDefaultsToCurrentBoard(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, textKey("9"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, textKey("E"))
	if !strings.Contains(m.View().Content, "Delivery (id=42)") {
		t.Fatalf("old board persisted: %q", m.View().Content)
	}
}

func TestExportNewBoardEntryStartsEmpty(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, textKey("9"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, textKey("8"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	if !strings.Contains(m.View().Content, "Board ID: 8\n") {
		t.Fatalf("new ID appended to old: %q", m.View().Content)
	}
}

func TestExportBoardChangeClearsPreviousSuccess(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	m.exportCopy = func(string) error { return nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, cmd = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, textKey("b"))
	if strings.Contains(m.View().Content, "Export copied") {
		t.Fatal("stale success after board scope change")
	}
}

func TestExportBoardInputRejectsNondigits(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, textKey("x"))
	if !strings.Contains(m.View().Content, "digits only") {
		t.Fatal("invalid board character silently ignored")
	}
}

func TestExportResolvesFirstActiveSprintBeforeCopy(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(id int) ([]jira.Sprint, error) {
		if id != 42 {
			t.Fatalf("sprint board = %d", id)
		}
		return []jira.Sprint{{ID: 7, Name: "Sprint Seven", State: "active"}, {ID: 8, State: "active"}}, nil
	}
	m, cmd := updateModel(t, m, textKey("E"))
	if cmd == nil {
		t.Fatal("default sprint resolution not requested")
	}
	m, _ = updateModel(t, m, cmd())
	if !strings.Contains(m.View().Content, "Sprint: 7") {
		t.Fatalf("active default not visible: %q", m.View().Content)
	}
}

func TestExportDoesNotCopyBeforeDefaultResolution(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(int) ([]jira.Sprint, error) { return nil, nil }
	m.exportFetch = func(int, int) ([]jira.Issue, error) { t.Fatal("fetched before resolution"); return nil, nil }
	m, _ = updateModel(t, m, textKey("E"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd != nil || !strings.Contains(m.View().Content, "Resolving") {
		t.Fatalf("resolution pending not shown or copy allowed: %q", m.View().Content)
	}
}

func TestExportSprintDiscoveryErrorPreventsCopy(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(int) ([]jira.Sprint, error) { return nil, errors.New("discovery failed") }
	m.exportFetch = func(int, int) ([]jira.Issue, error) { t.Fatal("fetch after failed discovery"); return nil, nil }
	m, cmd := updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, cmd())
	m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd != nil || !strings.Contains(m.View().Content, "discovery failed") {
		t.Fatalf("discovery error not enforced: %q", m.View().Content)
	}
}

func TestExportWarnsSprintDiscoveryCompletenessUnknown(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(int) ([]jira.Sprint, error) { return nil, nil }
	m, cmd := updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, cmd())
	if !strings.Contains(m.View().Content, "sprint discovery completeness unknown") {
		t.Fatalf("discovery warning missing: %q", m.View().Content)
	}
}

func TestExportBoardChangeResolvesNewDefault(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(board int) ([]jira.Sprint, error) {
		if board == 9 {
			return []jira.Sprint{{ID: 19, State: "active"}}, nil
		}
		return []jira.Sprint{{ID: 7, State: "active"}}, nil
	}
	m, cmd := updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, textKey("9"))
	m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("board change did not request sprint resolution")
	}
	m, _ = updateModel(t, m, cmd())
	if !strings.Contains(m.View().Content, "Sprint: 19") {
		t.Fatalf("new board default absent: %q", m.View().Content)
	}
}

func TestExportExplicitSprintOverridesPendingDefault(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(int) ([]jira.Sprint, error) { return []jira.Sprint{{ID: 7, State: "active"}}, nil }
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	m, cmd := updateModel(t, m, textKey("E"))
	old := cmd()
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, textKey("9"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, old)
	m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil || !strings.Contains(m.View().Content, "Sprint: 9") {
		t.Fatalf("explicit sprint did not override default: %q", m.View().Content)
	}
}

func TestExportExplicitAllBoardOverridesActiveSprint(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(int) ([]jira.Sprint, error) { return []jira.Sprint{{ID: 7, State: "active"}}, nil }
	var sprint int
	m.exportFetch = func(_ int, s int) ([]jira.Issue, error) { sprint = s; return nil, nil }
	m, cmd := updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, textKey("a"))
	m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil || !strings.Contains(m.View().Content, "All board issues") {
		t.Fatalf("all-board scope unavailable: %q", m.View().Content)
	}
	cmd()
	if sprint != 0 {
		t.Fatalf("sprint = %d, want all-board", sprint)
	}
}

func TestExportCancellingSprintEntryRetainsPendingResolution(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(int) ([]jira.Sprint, error) { return []jira.Sprint{{ID: 7, State: "active"}}, nil }
	m, cmd := updateModel(t, m, textKey("E"))
	old := cmd()
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, old)
	if !strings.Contains(m.View().Content, "Sprint: 7") {
		t.Fatalf("pending resolution lost after cancel: %q", m.View().Content)
	}
}

func TestExportCancellingBoardEntryRetainsPendingResolution(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(int) ([]jira.Sprint, error) { return []jira.Sprint{{ID: 7, State: "active"}}, nil }
	m, cmd := updateModel(t, m, textKey("E"))
	old := cmd()
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, old)
	if !strings.Contains(m.View().Content, "Sprint: 7") {
		t.Fatalf("pending resolution lost after board cancel: %q", m.View().Content)
	}
}

func TestExportExplicitSprintSurvivesBoardDiscoveryFailure(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(board int) ([]jira.Sprint, error) {
		if board == 9 {
			return nil, errors.New("discovery failed")
		}
		return nil, nil
	}
	m.exportFetch = func(int, int) ([]jira.Issue, error) { return nil, nil }
	m, cmd := updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, textKey("7"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, textKey("9"))
	m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	_, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("explicit sprint blocked by discovery failure")
	}
}

func TestExportDiscardsOldBoardSprintResolution(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(board int) ([]jira.Sprint, error) {
		if board == 9 {
			return []jira.Sprint{{ID: 19, State: "active"}}, nil
		}
		return []jira.Sprint{{ID: 7, State: "active"}}, nil
	}
	m, cmd := updateModel(t, m, textKey("E"))
	old := cmd()
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, textKey("9"))
	m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, old)
	if !strings.Contains(m.View().Content, "Sprint: 19") {
		t.Fatalf("old board resolution replaced new: %q", m.View().Content)
	}
}

func TestExportDefaultsToAllBoardWhenNoActiveSprint(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(int) ([]jira.Sprint, error) { return []jira.Sprint{{ID: 7, State: "future"}}, nil }
	m, cmd := updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, cmd())
	if !strings.Contains(m.View().Content, "All board issues") {
		t.Fatalf("board fallback missing: %q", m.View().Content)
	}
}

func TestExportDiscoveryFailureDoesNotClaimAllBoardScope(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m.exportSprints = func(int) ([]jira.Sprint, error) { return nil, errors.New("failed") }
	m, cmd := updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, cmd())
	if strings.Contains(m.View().Content, "All board issues") {
		t.Fatalf("unresolved default claimed board fallback: %q", m.View().Content)
	}
}

func TestExportEscapeReturnsToBoard(t *testing.T) {
	m := New(nil, "example.test")
	m.view = viewKanban
	m.boards = []jira.Board{{ID: 42, Name: "Delivery"}}
	m.loadedBoardRequest = 1
	m, _ = updateModel(t, m, textKey("E"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewKanban {
		t.Fatal("escape did not return to the board")
	}
}
