package tui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"

	tea "charm.land/bubbletea/v2"
)

func TestSearchLimitEditorExplainsAllMode(t *testing.T) {
	m := Model{view: viewBoards}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	view := updated.(Model).View().Content
	if !strings.Contains(view, "1–10000 or all") {
		t.Fatalf("editor must explain accepted limits: %s", view)
	}
}

func TestSearchLimitCanBeChanged(t *testing.T) {
	m := Model{view: viewBoards}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	m = updated.(Model)
	for _, r := range "25" {
		updated, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if !strings.Contains(m.View().Content, "Limit: 25") {
		t.Fatalf("expected selected limit: %s", m.View().Content)
	}
}

func TestSearchLimitRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"0", "-1", "10001", "abc", "99999999999999999999999"} {
		t.Run(value, func(t *testing.T) {
			m := Model{view: viewBoards}
			updated, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
			m = updated.(Model)
			updated, _ = m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
			m = updated.(Model)
			for _, r := range value {
				updated, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
				m = updated.(Model)
			}
			updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			m = updated.(Model)
			if !strings.Contains(strings.ToLower(m.View().Content), "invalid") || m.searchLimit != 0 || !m.searchLimitEditing {
				t.Fatalf("invalid value must not change limit and must show an error: %s", m.View().Content)
			}
		})
	}
}

func TestSearchLimitRoutesToTextAndJQL(t *testing.T) {
	for _, jql := range []bool{false, true} {
		t.Run(map[bool]string{false: "text", true: "JQL"}[jql], func(t *testing.T) {
			var max, query string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				max = r.URL.Query().Get("maxResults")
				query = r.URL.Query().Get("jql")
				io.WriteString(w, `{"issues":[],"isLast":true}`)
			}))
			defer srv.Close()
			m := Model{view: viewBoards, client: jira.NewClient(srv.URL, "", "")}
			updated, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
			m = updated.(Model)
			updated, _ = m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
			m = updated.(Model)
			for _, r := range "25" {
				updated, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
				m = updated.(Model)
			}
			updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			m = updated.(Model)
			if jql {
				updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
				m = updated.(Model)
			}
			for _, r := range "hello" {
				updated, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
				m = updated.(Model)
			}
			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("missing search command")
			}
			msg := cmd()
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, c := range batch {
					c()
				}
			}
			want := `text ~ "hello"`
			if jql {
				want = "hello"
			}
			if max != "25" || query != want {
				t.Fatalf("max=%q jql=%q want 25 and %q", max, query, want)
			}
		})
	}
}

func TestSearchLimitAllCapsAndReportsMore(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			io.WriteString(w, `{"count":12000}`)
			return
		}
		requests++
		if r.URL.Query().Get("maxResults") != "100" {
			t.Errorf("page size %s", r.URL.Query().Get("maxResults"))
		}
		io.WriteString(w, `{"issues":[`+strings.TrimSuffix(strings.Repeat(`{},`, 100), ",")+`],"nextPageToken":"`+strconv.Itoa(requests)+`","isLast":false}`)
	}))
	defer srv.Close()
	m := Model{view: viewBoards, client: jira.NewClient(srv.URL, "", "")}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	m = updated.(Model)
	for _, r := range "all" {
		updated, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if !strings.Contains(m.View().Content, "Limit: all") {
		t.Fatalf("all mode not selected: %s", m.View().Content)
	}
	for _, r := range "hello" {
		updated, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(Model)
	}
	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	batch := cmd().(tea.BatchMsg)
	for _, c := range batch {
		if msg, ok := c().(searchMsg); ok {
			updated, _ = m.Update(msg)
			m = updated.(Model)
		}
	}
	if requests != 100 || !strings.Contains(m.View().Content, "10000 shown") || !strings.Contains(m.View().Content, "about 12000") {
		t.Fatalf("requests=%d view=%s", requests, m.View().Content)
	}
}

func TestSearchTruncationDoesNotClaimEstimateIsMinimum(t *testing.T) {
	estimate := 120
	m := Model{view: viewSearch, searchCompleted: true, searchTruncated: true, searchEstimated: &estimate, searchResults: make([]jira.Issue, 1)}
	view := m.View().Content
	if strings.Contains(view, "≥120") || !strings.Contains(view, "more results") || !strings.Contains(view, "about 120") {
		t.Fatalf("misleading truncation: %s", view)
	}
}

func TestSearchLimitAcceptsPastedValue(t *testing.T) {
	m := Model{view: viewBoards}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	m = updated.(Model)
	updated, _ = m.Update(tea.PasteMsg{Content: "100"})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if !strings.Contains(m.View().Content, "Limit: 100") {
		t.Fatalf("paste not accepted: %s", m.View().Content)
	}
}

func TestSearchLimitBoundariesAndCancel(t *testing.T) {
	for _, value := range []string{"1", "10000"} {
		t.Run(value, func(t *testing.T) {
			m := Model{view: viewBoards}
			updated, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
			m = updated.(Model)
			updated, _ = m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
			m = updated.(Model)
			for _, r := range value {
				updated, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
				m = updated.(Model)
			}
			updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			m = updated.(Model)
			if !strings.Contains(m.View().Content, "Limit: "+value) {
				t.Fatalf("boundary not accepted: %s", m.View().Content)
			}
			updated, _ = m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
			m = updated.(Model)
			updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
			m = updated.(Model)
			if !strings.Contains(m.View().Content, "Limit: "+value) {
				t.Fatalf("cancel changed limit: %s", m.View().Content)
			}
		})
	}
}

func TestSearchLimitIsDiscoverable(t *testing.T) {
	m := Model{view: viewBoards}
	updated, _ := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	view := updated.(Model).View().Content
	if !strings.Contains(view, "Limit: 50") || !strings.Contains(view, "ctrl+l") {
		t.Fatalf("search must show its default limit and shortcut: %s", view)
	}
}
