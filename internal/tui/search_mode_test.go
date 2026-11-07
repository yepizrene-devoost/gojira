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

func searchCommand(cmd tea.Cmd) tea.Msg {
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if result := c(); result != nil {
				if _, ok := result.(searchMsg); ok {
					return result
				}
			}
		}
	}
	return msg
}

func TestJQLSearchSendsUnchangedQuery(t *testing.T) {
	const query = `  project = ABC AND summary ~ "hello world"  `
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("jql")
		fmt.Fprint(w, `{"issues":[],"isLast":true}`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	m, _ = updateModel(t, m, tea.PasteMsg{Content: query})
	_, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	searchCommand(cmd)
	if got != query {
		t.Fatalf("JQL sent = %q, want unchanged %q", got, query)
	}
}

func TestSwitchingSearchModeRejectsPendingResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"issues":[{"key":"OLD-1","fields":{"summary":"old result"}}],"isLast":true}`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, tea.PasteMsg{Content: "old"})
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	old := searchCommand(cmd)
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	m, _ = updateModel(t, m, old)
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if strings.Contains(m.View().Content, "OLD-1") {
		t.Fatal("stale response appeared after switching modes")
	}
}

func TestJQLSearchHTTPCompleteness(t *testing.T) {
	for _, tc := range []struct {
		name        string
		last        bool
		countStatus int
		want        string
		absent      string
	}{
		{name: "complete at limit", last: true, want: "50 shown", absent: "more results"},
		{name: "truncated with estimate", countStatus: http.StatusOK, want: "50 shown · ≥75 estimated"},
		{name: "truncated without estimate", countStatus: http.StatusBadRequest, want: "50 shown · more results", absent: "estimated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var searches, counts int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					counts++
					if tc.countStatus != http.StatusOK {
						w.WriteHeader(tc.countStatus)
						return
					}
					fmt.Fprint(w, `{"count":75}`)
					return
				}
				searches++
				if got := r.URL.Query().Get("maxResults"); got != "50" {
					t.Errorf("maxResults = %q", got)
				}
				issues := make([]map[string]any, 50)
				for i := range issues {
					issues[i] = map[string]any{"key": fmt.Sprintf("T-%d", i+1), "fields": map[string]any{"summary": "test"}}
				}
				response := map[string]any{"issues": issues, "isLast": tc.last}
				if !tc.last {
					response["nextPageToken"] = "next"
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Error(err)
				}
			}))
			defer srv.Close()
			m := New(jira.NewClient(srv.URL, "test", "test"), "")
			m, _ = updateModel(t, m, textKey("s"))
			m, _ = updateModel(t, m, specialKey(tea.KeyTab))
			m, _ = updateModel(t, m, tea.PasteMsg{Content: "project = T"})
			m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
			m, _ = updateModel(t, m, searchCommand(cmd))
			out := ansi.Strip(m.View().Content)
			if !strings.Contains(out, tc.want) || (tc.absent != "" && strings.Contains(out, tc.absent)) {
				t.Fatalf("unexpected completeness: %s", out)
			}
			wantCounts := 0
			if !tc.last {
				wantCounts = 1
			}
			if searches != 1 || counts != wantCounts {
				t.Fatalf("searches=%d counts=%d", searches, counts)
			}
		})
	}
}

func TestJQLSearchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"errorMessages":["invalid JQL"]}`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	m, _ = updateModel(t, m, tea.PasteMsg{Content: "invalid"})
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, searchCommand(cmd))
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Error:") || strings.Contains(out, "No tickets found") || strings.Contains(out, "shown") {
		t.Fatalf("error presented as results: %s", out)
	}
}

func TestTextSearchHTTPRegression(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("jql")
		fmt.Fprint(w, `{"issues":[{"key":"T-1","fields":{"summary":"found"}}],"isLast":true}`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, tea.PasteMsg{Content: `  hello "world"  `})
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, searchCommand(cmd))
	if got != `text ~ "hello \"world\""` || !strings.Contains(ansi.Strip(m.View().Content), "T-1") {
		t.Fatalf("text JQL=%q output=%s", got, m.View().Content)
	}
}

func TestFilterSearchEditingHasTextPlaceholder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"issues":[],"isLast":true}`) }))
	defer srv.Close()
	m := makeTestModel(1, 1)
	m.client = jira.NewClient(srv.URL, "test", "test")
	m, _ = updateModel(t, m, textKey("/"))
	m, _ = updateModel(t, m, tea.PasteMsg{Content: "hello"})
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, searchCommand(cmd))
	m, _ = updateModel(t, m, textKey("s"))
	for i := 0; i < 5; i++ {
		m, _ = updateModel(t, m, specialKey(tea.KeyBackspace))
	}
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "Jira text search") {
		t.Fatalf("filter search placeholder incorrect: %s", out)
	}
}

func TestCancelledModeRestoresPlaceholder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"issues":[],"isLast":true}`) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, tea.PasteMsg{Content: "hello"})
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, searchCommand(cmd))
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, textKey("s"))
	for i := 0; i < 5; i++ {
		m, _ = updateModel(t, m, specialKey(tea.KeyBackspace))
	}
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "Jira text search") {
		t.Fatalf("restored text placeholder incorrect: %s", out)
	}
}

func TestKanbanHelpDiscoversJQLSearch(t *testing.T) {
	m := makeTestModel(1, 1)
	m, _ = updateModel(t, m, textKey("?"))
	if out := ansi.Strip(m.View().Content); !strings.Contains(out, "JQL") {
		t.Fatalf("help does not identify JQL search: %s", out)
	}
}

func TestSearchPlaceholderTracksMode(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	if got := ansi.Strip(m.View().Content); !strings.Contains(got, "Jira JQL search") || strings.Contains(got, "Jira text search") {
		t.Fatalf("JQL placeholder incorrect: %s", got)
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	if got := ansi.Strip(m.View().Content); !strings.Contains(got, "Jira text search") {
		t.Fatalf("text placeholder incorrect: %s", got)
	}
}

func TestSwitchingSearchModeRejectsPendingError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"errorMessages":["old failure"]}`)
	}))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, tea.PasteMsg{Content: "old"})
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	old := searchCommand(cmd)
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	m, _ = updateModel(t, m, old)
	if out := ansi.Strip(m.View().Content); strings.Contains(out, "Error:") {
		t.Fatalf("stale error appeared: %s", out)
	}
}

func TestJQLEditingHelpIdentifiesJQL(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	if strings.Contains(m.View().Content, "Type a text query") {
		t.Fatal("JQL mode still prompts for text query")
	}
}

func TestCancellingModeChangeRestoresResultMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"issues":[],"isLast":true}`) }))
	defer srv.Close()
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, tea.PasteMsg{Content: "hello"})
	m, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	m, _ = updateModel(t, m, searchCommand(cmd))
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	if !strings.Contains(m.View().Content, "Mode: Text") {
		t.Fatal("cancel did not restore result mode")
	}
}

func TestFilterSearchRemainsTextAfterJQL(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("jql")
		fmt.Fprint(w, `{"issues":[],"isLast":true}`)
	}))
	defer srv.Close()
	m := makeTestModel(1, 1)
	m.client = jira.NewClient(srv.URL, "test", "test")
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, textKey("/"))
	m, _ = updateModel(t, m, tea.PasteMsg{Content: "login"})
	_, cmd := updateModel(t, m, specialKey(tea.KeyEnter))
	searchCommand(cmd)
	if got != `text ~ "login"` {
		t.Fatalf("filter search JQL = %q", got)
	}
}

func TestReopeningSearchDefaultsToText(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("s"))
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	m, _ = updateModel(t, m, specialKey(tea.KeyEsc))
	m, _ = updateModel(t, m, textKey("s"))
	if !strings.Contains(m.View().Content, "Mode: Text") {
		t.Fatal("reopened search did not default to Text")
	}
}

func TestSearchModeIsDiscoverableAndDefaultsToText(t *testing.T) {
	m := New(nil, "")
	m, _ = updateModel(t, m, textKey("s"))
	output := m.View().Content
	if !strings.Contains(output, "Text") || !strings.Contains(output, "Tab") {
		t.Fatalf("search must show the default Text mode and its Tab toggle: %s", output)
	}
	m, _ = updateModel(t, m, specialKey(tea.KeyTab))
	if output = m.View().Content; !strings.Contains(output, "JQL") {
		t.Fatalf("Tab must select JQL mode: %s", output)
	}
}
