package tui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yepizrene-devoost/gojira/internal/jira"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestTimeReportDiscoverableFromBoards(t *testing.T) {
	m := New(nil, "")
	if !strings.Contains(strings.ToLower(ansi.Strip(m.View().Content)), "time report") {
		t.Fatal("boards do not advertise time report")
	}
}

func TestTimeReportAccountFilterAcceptsTyping(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("r"))
	m, _ = updateModel(t, m, textKey("x"))
	if !strings.Contains(ansi.Strip(m.View().Content), "x") {
		t.Fatal("account filter did not accept typed input")
	}
}

func TestTimeReportTabEditsProject(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("r"))
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	m, _ = updateModel(t, m, textKey("x"))
	if !strings.Contains(ansi.Strip(m.View().Content), "Project: x") {
		t.Fatalf("project filter did not accept input: %s", ansi.Strip(m.View().Content))
	}
}

func TestTimeReportTabEditsDateAndTimezoneFilters(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("r"))
	for _, want := range []string{"From: x", "To: y", "Timezone: z"} {
		m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		if strings.HasPrefix(want, "From:") {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		value := strings.TrimPrefix(want, want[:len(want)-1])
		m, _ = updateModel(t, m, textKey(value))
		if !strings.Contains(ansi.Strip(m.View().Content), want) {
			t.Fatalf("missing %q: %s", want, ansi.Strip(m.View().Content))
		}
	}
}

func TestTimeReportRejectsEmptyFilters(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("r"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("invalid filters started a request")
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "required") {
		t.Fatalf("missing validation error: %s", ansi.Strip(m.View().Content))
	}
}

func TestTimeReportValidSubmissionRequestsProject(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.URL.Query().Get("jql"); got != `project = "ABC" ORDER BY key ASC` {
			t.Errorf("jql = %q", got)
		}
		_, _ = fmt.Fprint(w, `{"issues":[],"isLast":true}`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	_, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("valid filters did not start report")
	}
	cmd()
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestTimeReportDisplaysResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"issues":[],"isLast":true}`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("valid submission did not start")
	}
	m, _ = updateModel(t, m, cmd())
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Total: 0") || !strings.Contains(out, "complete") {
		t.Fatalf("missing total or completeness: %s", out)
	}
}

func TestTimeReportDisplaysPartialWarnings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, `{"issues":[]}`) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("valid submission did not start")
	}
	m, _ = updateModel(t, m, cmd())
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"partial", "Invalid issue paging metadata", "permissions may hide data"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
}

func TestTimeReportDisplaysRequestError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "failure", 500) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("valid submission did not start")
	}
	m, _ = updateModel(t, m, cmd())
	if !strings.Contains(ansi.Strip(m.View().Content), "report scan failed") {
		t.Fatal("missing request error")
	}
}

func TestTimeReportDisplaysIssueRows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "worklog") {
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"worklogs":[{"id":"w1","author":{"accountId":"account"},"started":"2025-01-02T00:00:00Z","timeSpentSeconds":60}]}`)
		} else {
			_, _ = fmt.Fprint(w, `{"issues":[{"id":"1","key":"ABC-1"}],"isLast":true}`)
		}
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("valid submission did not start")
	}
	m, _ = updateModel(t, m, cmd())
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "ABC-1") || !strings.Contains(out, "60") {
		t.Fatalf("missing issue row: %s", out)
	}
}

func TestTimeReportEscapeCancelsHTTP(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("valid submission did not start")
	}
	go cmd()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	_, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		srv.CloseClientConnections()
		t.Fatal("escape did not cancel HTTP request")
	}
}

func TestTimeReportRejectsLateResponseAfterEscape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, `{"issues":[],"isLast":true}`) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("valid submission did not start")
	}
	late := cmd()
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, textKey("r"))
	m, _ = updateModel(t, m, late)
	if strings.Contains(ansi.Strip(m.View().Content), "Total:") {
		t.Fatal("late result leaked into new report")
	}
}

func TestTimeReportScrollsToLastRow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "worklog") {
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":1,"worklogs":[{"id":"w`+strings.Split(r.URL.Path, "/")[5]+`","author":{"accountId":"account"},"started":"2025-01-02T00:00:00Z","timeSpentSeconds":60}]}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"issues":[`)
		for i := 1; i <= 12; i++ {
			if i > 1 {
				_, _ = fmt.Fprint(w, ",")
			}
			_, _ = fmt.Fprintf(w, `{"id":"%d","key":"ABC-%02d"}`, i, i)
		}
		_, _ = fmt.Fprint(w, `],"isLast":true}`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 8})
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("no request")
	}
	m, _ = updateModel(t, m, cmd())
	if strings.Contains(ansi.Strip(m.View().Content), "ABC-12") {
		t.Fatal("last row initially visible despite small window")
	}
	for i := 0; i < 20; i++ {
		m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "ABC-12") {
		t.Fatalf("last row inaccessible: %s", ansi.Strip(m.View().Content))
	}
}

func TestTimeReportInvalidFiltersMakeNoHTTPRequests(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd != nil {
		cmd()
	}
	if calls != 0 {
		t.Fatalf("invalid filters issued %d requests", calls)
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "required") {
		t.Fatal("missing validation error")
	}
}

func TestTimeReportNewRequestRejectsOldResult(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_, _ = fmt.Fprint(w, `{"issues":[]}`)
		} else {
			_, _ = fmt.Fprint(w, `{"issues":[],"isLast":true}`)
		}
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, first := updateModel(t, m, specialKey(tea.KeyEnter))
	if first == nil {
		t.Fatal("missing first command")
	}
	old := first()
	m, second := updateModel(t, m, specialKey(tea.KeyEnter))
	if second == nil {
		t.Fatal("missing second command")
	}
	m, _ = updateModel(t, m, second())
	m, _ = updateModel(t, m, old)
	if strings.Contains(ansi.Strip(m.View().Content), "Invalid issue paging metadata") {
		t.Fatal("old result replaced new result")
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "complete") {
		t.Fatal("new result missing")
	}
}

func TestTimeReportAdvertisedInKanbanHelp(t *testing.T) {
	m := New(nil, "")
	m.view = viewKanban
	m, _ = updateModel(t, m, textKey("?"))
	if !strings.Contains(strings.ToLower(ansi.Strip(m.View().Content)), "time report") {
		t.Fatal("kanban help omits time report")
	}
}

func TestTimeReportNarrowWindowShowsAllWarnings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, `{"issues":[]}`) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 30, Height: 6})
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("missing report command")
	}
	m, _ = updateModel(t, m, cmd())
	seen := ""
	for i := 0; i < 50; i++ {
		out := ansi.Strip(m.View().Content)
		for _, line := range strings.Split(out, "\n") {
			if len([]rune(line)) > 30 {
				t.Fatalf("line exceeds terminal width: %q", line)
			}
		}
		seen += " " + strings.ReplaceAll(out, "\n", " ")
		m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	}
	for _, phrase := range []string{"Only issues and worklogs", "permissions may hide data", "not a transactional snapshot", "Invalid issue paging metadata", "totals are partial"} {
		if !strings.Contains(seen, phrase) {
			t.Fatalf("warning text %q not reachable: %s", phrase, seen)
		}
	}
}

func TestTimeReportInvalidResubmissionShowsValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, `{"issues":[],"isLast":true}`) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("missing request")
	}
	m, _ = updateModel(t, m, cmd())
	for i := 0; i < 3; i++ {
		m, _ = updateModel(t, m, specialKey(tea.KeyBackspace))
	}
	m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("invalid resubmission started request")
	}
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "required") || strings.Contains(out, "Total:") {
		t.Fatalf("validation hidden or prior report misleading: %s", out)
	}
}

func TestTimeReportResubmissionHidesPreviousResultWhilePending(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, `{"issues":[],"isLast":true}`) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("missing request")
	}
	m, _ = updateModel(t, m, cmd())
	m, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("missing second request")
	}
	out := ansi.Strip(m.View().Content)
	if strings.Contains(out, "Total:") || !strings.Contains(out, "Loading") {
		t.Fatalf("previous report still presented while pending: %s", out)
	}
}

func TestTimeReportNewSubmissionCancelsPreviousHTTP(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(cancelled) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("missing request")
	}
	go cmd()
	select {
	case <-started:
	case <-time.After(time.Second):
		srv.CloseClientConnections()
		t.Fatal("request did not start")
	}
	_, cmd = updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		srv.CloseClientConnections()
		t.Fatal("missing second request")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		srv.CloseClientConnections()
		t.Fatal("previous HTTP request was not cancelled")
	}
}

func TestTimeReportInvalidResubmissionRejectsPendingResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, `{"issues":[],"isLast":true}`) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("missing request")
	}
	late := cmd()
	for i := 0; i < 3; i++ {
		m, _ = updateModel(t, m, specialKey(tea.KeyBackspace))
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, late)
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "required") || strings.Contains(out, "Total:") || strings.Contains(out, "Loading") {
		t.Fatalf("late report overwrote validation: %s", out)
	}
}

func TestTimeReportValidationVisibleInNarrowWindow(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 20, Height: 6})
	m, _ = updateModel(t, m, textKey("r"))
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	seen := ""
	for i := 0; i < 20; i++ {
		out := ansi.Strip(m.View().Content)
		for _, line := range strings.Split(out, "\n") {
			if len([]rune(line)) > 20 {
				t.Fatalf("validation line exceeds width: %q", line)
			}
		}
		seen += " " + strings.ReplaceAll(out, "\n", " ")
		m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	}
	for _, part := range []string{"account ID", "project key", "timezone", "required"} {
		if !strings.Contains(seen, part) {
			t.Fatalf("missing validation %q", part)
		}
	}
}

func TestTimeReportValidationVisibleAfterScrolledResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, `{"issues":[]}`) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 30, Height: 4})
	m, _ = updateModel(t, m, textKey("r"))
	for i, value := range []string{"account", "ABC", "2025-01-01", "2025-01-31", "UTC"} {
		if i > 0 {
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
		}
		m, _ = updateModel(t, m, textKey(value))
	}
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("missing report command")
	}
	m, _ = updateModel(t, m, cmd())
	for i := 0; i < 20; i++ {
		m, _ = updateModel(t, m, specialKey(tea.KeyDown))
	}
	for i := 0; i < 3; i++ {
		m, _ = updateModel(t, m, specialKey(tea.KeyBackspace))
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyEnter))
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "account ID") {
		t.Fatalf("validation not immediately visible after scrolling: %s", out)
	}
}

func TestTimeReportShortcutOpensFilterScreen(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("r"))
	out := strings.ToLower(ansi.Strip(m.View().Content))
	for _, want := range []string{"account", "project", "from", "to", "timezone"} {
		if !strings.Contains(out, want) {
			t.Fatalf("time report filter screen missing %q: %s", want, out)
		}
	}
}
