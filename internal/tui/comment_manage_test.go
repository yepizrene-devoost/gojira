package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func mustDecodeBody(r *http.Request, into any) error {
	return json.NewDecoder(r.Body).Decode(into)
}

// ─── helpers ─────────────────────────────────────────────────────────────

func twoComments() []jira.Comment {
	first := jira.Comment{ID: "10001", Created: "2026-11-09T10:00:00.000+0000"}
	first.Body = jira.TextToADF("Reviewed the API")
	first.Author.DisplayName = "Ada"
	second := jira.Comment{ID: "10002", Created: "2026-11-09T11:00:00.000+0000"}
	second.Body = jira.TextToADF("Looks good")
	second.Author.DisplayName = "Bob"
	return []jira.Comment{first, second}
}

// openCommentList presses `C` and delivers the collection reply the way the
// runtime does: the model starts one scoped read, and the reply carries that
// request id.
func openCommentList(t *testing.T, m Model, comments []jira.Comment) Model {
	t.Helper()
	m, cmd := updateModel(t, m, textKey("C"))
	if m.view != viewCommentList {
		t.Fatalf("C did not open the comment list: view=%v", m.view)
	}
	if cmd == nil {
		t.Fatal("C did not start a comment collection read")
	}
	m, _ = updateModel(t, m, commentListMsg{
		request:  m.activeCommentListRequest,
		issueKey: m.commentIssue,
		comments: comments,
	})
	if m.commentListLoading {
		t.Fatal("collection reply did not finish the load")
	}
	return m
}

func commentManageModel() Model {
	issue := testIssue("P0")
	m := makeTestModel(1, 1)
	m.view = viewDetail
	m.detail = &issue
	m.detailIssueKey = issue.Key
	m.detailBoardRequest = 4
	m.loadedBoardRequest = 4
	return m
}

// ─── S1: the manage view reads the full collection ────────────────────────

// `C` opens a selectable list of the issue's comments with ids, authors and
// bodies, and esc returns to detail.
func TestCommentListShowsEntriesAndMovesSelection(t *testing.T) {
	m := openCommentList(t, commentManageModel(), twoComments())
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"Comments: P0", "10001", "Ada", "Reviewed the API", "10002", "Bob", "Looks good", "[↑↓] Select  [e] Edit  [d] Delete  [esc] Back"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list is missing %q:\n%s", want, out)
		}
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	if m.commentCur != 1 {
		t.Fatalf("down did not move the selection: %d", m.commentCur)
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewDetail {
		t.Fatalf("esc did not return to detail: view=%v", m.view)
	}
}

// An issue without comments says so instead of showing an empty box.
func TestCommentListEmptyState(t *testing.T) {
	m := openCommentList(t, commentManageModel(), nil)
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "No comments to manage") {
		t.Fatalf("missing empty state:\n%s", out)
	}
}

// A failed collection read surfaces inside the list; it never closes the view.
func TestCommentListErrorState(t *testing.T) {
	m, cmd := updateModel(t, commentManageModel(), textKey("C"))
	if cmd == nil {
		t.Fatal("C did not start a comment collection read")
	}
	m, _ = updateModel(t, m, commentListMsg{
		request:  m.activeCommentListRequest,
		issueKey: m.commentIssue,
		err:      fmt.Errorf("permission denied"),
	})
	out := ansi.Strip(m.View().Content)
	if m.view != viewCommentList || !strings.Contains(out, "permission denied") {
		t.Fatalf("error state: view=%v\n%s", m.view, out)
	}
}

// ─── S1/S3: editing prefills and replaces the body as ADF ────────────────

func TestCommentEditPrefillsAndSubmitsADF(t *testing.T) {
	comments := twoComments()
	m := openCommentList(t, commentManageModel(), comments)
	m, _ = updateModel(t, m, textKey("e"))
	if m.view != viewCommentEdit {
		t.Fatalf("e did not open the comment editor: view=%v", m.view)
	}
	// The editor header states the ADF replacement semantics.
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "replaces the whole body") || !strings.Contains(out, "plain text") {
		t.Fatalf("editor does not state ADF semantics:\n%s", out)
	}
	if strings.TrimSpace(m.commentEditInput.Value()) != "Reviewed the API" {
		t.Fatalf("editor was not prefilled: %q", m.commentEditInput.Value())
	}

	var method, path string
	var sent struct {
		Body jira.ADFDoc `json:"body"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		if r.Method == http.MethodPut {
			_ = mustDecodeBody(r, &sent)
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	m.client = jira.NewClient(srv.URL, "email", "token")
	m.commentEditInput.SetValue("Edited text")
	m, cmd := updateModel(t, m, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+s did not submit the edit")
	}
	if m.view != viewCommentEdit {
		t.Fatalf("submit left the editor: view=%v", m.view)
	}
	// A second ctrl+s must not fire twice.
	if _, duplicate := updateModel(t, m, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}); duplicate != nil {
		t.Fatal("double submit")
	}
	result := cmd()
	// The reply lands while the editor is still open, the way the runtime does.
	updated, _ := updateModel(t, m, result)
	m = updated
	if m.view != viewCommentList {
		t.Fatalf("done message did not return to the list: view=%v", m.view)
	}
	if m.commentMutationWarning == nil || !strings.Contains(m.commentMutationWarning.Error(), "plain text") {
		t.Fatalf("missing ADF replacement warning: %v", m.commentMutationWarning)
	}
	if method != http.MethodPut || path != "/rest/api/3/issue/P0/comment/10001" {
		t.Fatalf("request=%s %s", method, path)
	}
	if sent.Body.Flatten() != "Edited text" {
		t.Fatalf("sent body=%q", sent.Body.Flatten())
	}
}

// A blank edit never reaches Jira.
func TestCommentEditRejectsBlank(t *testing.T) {
	m := openCommentList(t, commentManageModel(), twoComments())
	m, _ = updateModel(t, m, textKey("e"))
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			posts++
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	m.client = jira.NewClient(srv.URL, "email", "token")
	m.commentEditInput.SetValue("  ")
	m, cmd := updateModel(t, m, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if cmd != nil || posts != 0 || m.err == nil {
		t.Fatalf("blank edit accepted: cmd=%v posts=%d err=%v", cmd, posts, m.err)
	}
}

// ─── S4: delete is destructive and needs `y` ─────────────────────────────

func TestCommentDeleteNeedsConfirmation(t *testing.T) {
	comments := twoComments()
	m := openCommentList(t, commentManageModel(), comments)
	m, _ = updateModel(t, m, textKey("d"))
	if m.view != viewCommentDelete {
		t.Fatalf("d did not open the delete confirmation: view=%v", m.view)
	}
	deletes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deletes++
			w.WriteHeader(204)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	m.client = jira.NewClient(srv.URL, "email", "token")
	// esc cancels without any request.
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if m.view != viewCommentList || deletes != 0 {
		t.Fatalf("esc did not cancel: view=%v deletes=%d", m.view, deletes)
	}
	// Reopen and confirm with y.
	m, _ = updateModel(t, m, textKey("d"))
	m, cmd := updateModel(t, m, textKey("y"))
	if cmd == nil {
		t.Fatal("y did not submit the delete")
	}
	result := cmd()
	updated, _ := updateModel(t, m, result)
	m = updated
	if deletes != 1 {
		t.Fatalf("deletes=%d", deletes)
	}
	if m.view != viewCommentList {
		t.Fatalf("done message did not return to the list: view=%v", m.view)
	}
}

// ─── S2: the detail hint and the help advertise the manage view ──────────

func TestDetailAdvertisesCommentManage(t *testing.T) {
	m := commentManageModel()
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "[C] Comments") {
		t.Fatalf("detail hint missing [C] Comments:\n%s", out)
	}
	found := false
	for _, entry := range kanbanHelp {
		if strings.Contains(entry.action, "C lists/edits/deletes them") {
			found = true
		}
	}
	if !found {
		t.Fatal("kanban help missing C (detail)")
	}
}
