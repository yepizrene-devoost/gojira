package tui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestUserSearchDiscoverableFromBoards(t *testing.T) {
	m := New(nil, "")
	m.boards = []jira.Board{{Name: "Board"}}
	if !strings.Contains(strings.ToLower(ansi.Strip(m.View().Content)), "user") {
		t.Fatal("boards do not advertise user search")
	}
	m, _ = updateModel(t, m, textKey("u"))
	if !strings.Contains(strings.ToLower(ansi.Strip(m.View().Content)), "user search") {
		t.Fatal("user search shortcut did not open user search")
	}
}

func TestUserSearchUsesJiraUserEndpoint(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/rest/api/3/user/search" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "alice" {
			t.Errorf("query = %q", r.URL.Query().Get("query"))
		}
		_, _ = fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, _ = updateModel(t, m, textKey("l"))
	m, _ = updateModel(t, m, textKey("i"))
	m, _ = updateModel(t, m, textKey("c"))
	m, _ = updateModel(t, m, textKey("e"))
	_, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("valid query did not start request")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			c()
		}
	}
	if calls != 1 {
		t.Fatalf("requests = %d, want 1", calls)
	}
}

func TestUserSearchDisplaysIdentity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[{"accountId":"id-1","displayName":"Alice Example","active":true}]`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"Alice Example", "id-1", "active"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
}

func TestUserSearchEmptyCompletenessWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, `[]`) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"endpoint_window_exhausted", "Browse users permission", "No users"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
}

func TestUserSearchShowsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "failed", http.StatusInternalServerError) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	if !strings.Contains(ansi.Strip(m.View().Content), "user search failed") {
		t.Fatal("missing search error")
	}
}

func TestUserSearchEscapeRejectsLateResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[{"accountId":"late","displayName":"Late User","active":true}]`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	late := cmd()
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, late)
	m, _ = updateModel(t, m, textKey("u"))
	if strings.Contains(ansi.Strip(m.View().Content), "Late User") {
		t.Fatal("late result leaked into new search")
	}
}

func TestUserSearchShowsLoading(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	if !strings.Contains(ansi.Strip(m.View().Content), "Searching users") {
		t.Fatal("missing loading status")
	}
}

func TestUserSearchPartialWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("startAt") != "0" {
			http.Error(w, "failed", 500)
			return
		}
		_, _ = fmt.Fprint(w, "[")
		for i := 0; i < 100; i++ {
			if i > 0 {
				_, _ = fmt.Fprint(w, ",")
			}
			_, _ = fmt.Fprintf(w, `{"accountId":"%d","displayName":"User","active":true}`, i)
		}
		_, _ = fmt.Fprint(w, "]")
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "partial") || !strings.Contains(out, "interrupted") {
		t.Fatalf("missing partial warning: %s", out)
	}
}

func TestUserSearchDiscoverableFromKanban(t *testing.T) {
	m := New(nil, "")
	m.view = viewKanban
	m, _ = updateModel(t, m, textKey("?"))
	if !strings.Contains(strings.ToLower(ansi.Strip(m.View().Content)), "user search") {
		t.Fatal("help omits user search")
	}
	m, _ = updateModel(t, m, textKey("?"))
	m, _ = updateModel(t, m, textKey("u"))
	if !strings.Contains(strings.ToLower(ansi.Strip(m.View().Content)), "user search") {
		t.Fatal("shortcut did not open user search")
	}
}

func TestUserSearchPaste(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, tea.PasteMsg{Content: "Alice Example"})
	if !strings.Contains(ansi.Strip(m.View().Content), "Alice Example") {
		t.Fatal("pasted query is missing")
	}
}

func TestUserSearchEditingRejectsPendingResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[{"accountId":"old","displayName":"Old Result","active":true}]`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	late := cmd()
	m, _ = updateModel(t, m, textKey("b"))
	m, _ = updateModel(t, m, late)
	if strings.Contains(ansi.Strip(m.View().Content), "Old Result") {
		t.Fatal("old query result shown after editing")
	}
}

func TestUserSearchShowsControls(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("u"))
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"Enter", "Esc"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing control %s", want)
		}
	}
}

func TestUserSearchResultScrolling(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[{"accountId":"1","displayName":"First","active":true},{"accountId":"2","displayName":"Second","active":true}]`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m.height = 8
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	if !strings.Contains(ansi.Strip(m.View().Content), "→ Second") {
		t.Fatal("cannot scroll to second result")
	}
}

func TestUserSearchReopenResetsSelection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[{"accountId":"1","displayName":"First","active":true},{"accountId":"2","displayName":"Second","active":true}]`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	if !strings.Contains(ansi.Strip(m.View().Content), "→ First") {
		t.Fatal("selection not reset on reopening")
	}
}

func TestUserSearchErrorDoesNotClaimEmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "failed", 500) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, cmd())
	if strings.Contains(ansi.Strip(m.View().Content), "No users found") {
		t.Fatal("failure presented as an empty result")
	}
}

func TestUserSearchEscapeCancelsHTTP(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
			close(cancelled)
		case <-time.After(3 * time.Second):
		}
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("u"))
	m, _ = updateModel(t, m, textKey("a"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	done := make(chan struct{})
	go func() { cmd(); close(done) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not start")
	}
	_, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("request was not canceled")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("command did not return")
	}
}

func TestUserSearchRequiresQuery(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("u"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("empty query started a request")
	}
	if !strings.Contains(strings.ToLower(ansi.Strip(m.View().Content)), "query is required") {
		t.Fatal("missing required-query validation")
	}
}
