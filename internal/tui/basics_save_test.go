package tui

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// basicsJira serves TEST-1: a PUT records the body, a GET answers with the refreshed ticket.
type basicsJira struct {
	putStatus, getStatus int
	refreshed            string
	puts, gets           atomic.Int32
	body                 atomic.Value
}

func newBasicsJira() *basicsJira {
	return &basicsJira{
		putStatus: http.StatusNoContent,
		getStatus: http.StatusOK,
		refreshed: `{"key":"TEST-1","fields":{"summary":"New summary","priority":{"name":"High"},"labels":["a","b"]}}`,
	}
}

func (j *basicsJira) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/TEST-1" {
			t.Errorf("unexpected path %s %s", r.Method, r.URL.Path)
			return
		}
		switch r.Method {
		case http.MethodPut:
			j.puts.Add(1)
			raw, _ := io.ReadAll(r.Body)
			j.body.Store(strings.TrimSpace(string(raw)))
			w.WriteHeader(j.putStatus)
		case http.MethodGet:
			j.gets.Add(1)
			w.WriteHeader(j.getStatus)
			if j.getStatus == http.StatusOK {
				_, _ = fmt.Fprint(w, j.refreshed)
			}
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}
}

func (j *basicsJira) sentBody() string { s, _ := j.body.Load().(string); return s }

// basicsSet focuses the given input (0 summary, 1 priority, 2 labels) from the first one,
// clears it and types text.
func basicsSet(m Model, input int, text string) Model {
	for i := 0; i < input; i++ {
		m, _ = basicsPress(m, "tab")
	}
	for i := 0; i < 80; i++ {
		m, _ = basicsPress(m, "backspace")
	}
	m = basicsType(m, text)
	for i := 0; i < (3-input)%3; i++ {
		m, _ = basicsPress(m, "tab")
	}
	return m
}

func basicsDeliver(m Model, cmd tea.Cmd) Model {
	next, _ := m.Update(cmd())
	return next.(Model)
}

func openBasics(t *testing.T, j *basicsJira) Model {
	t.Helper()
	m, _ := basicsModel(t, viewKanban, j.handler(t))
	m, _ = basicsPress(m, "U")
	return m
}

func TestBasicsSavesOnlyTheChangedFieldsWithTheSharedPayload(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(Model) Model
		body string
	}{
		{"summary only", func(m Model) Model { return basicsSet(m, 0, "New summary") }, `{"fields":{"summary":"New summary"}}`},
		{"priority only", func(m Model) Model { return basicsSet(m, 1, "High") }, `{"fields":{"priority":{"name":"High"}}}`},
		{"labels only", func(m Model) Model { return basicsSet(m, 2, "a, b") }, `{"fields":{"labels":["a","b"]}}`},
		{"all three", func(m Model) Model { return basicsSet(basicsSet(basicsSet(m, 0, "New summary"), 1, "High"), 2, "a,b") }, `{"fields":{"labels":["a","b"],"priority":{"name":"High"},"summary":"New summary"}}`},
		{"values are trimmed", func(m Model) Model { return basicsSet(m, 0, "  New summary  ") }, `{"fields":{"summary":"New summary"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := newBasicsJira()
			m := tc.edit(openBasics(t, j))
			m, cmd := basicsPress(m, "ctrl+s")
			if cmd == nil {
				t.Fatalf("Ctrl+S sent nothing:\n%s", m.View().Content)
			}
			m = basicsDeliver(m, cmd)
			if j.puts.Load() != 1 || j.gets.Load() != 1 || j.sentBody() != tc.body {
				t.Fatalf("puts=%d gets=%d body=%s, want one PUT %s and one refresh", j.puts.Load(), j.gets.Load(), j.sentBody(), tc.body)
			}
			if m.view != viewDetail || m.detail == nil || m.detail.Fields.Summary != "New summary" || m.toast != "Ticket fields updated" {
				t.Fatalf("view=%v detail=%+v toast=%q", m.view, m.detail, m.toast)
			}
			if strings.Contains(m.View().Content, "Edit summary, priority and labels") {
				t.Fatal("the form is still visible after a successful save")
			}
		})
	}
}

func TestBasicsWithoutChangesClosesWithoutAnyRequest(t *testing.T) {
	for name, edit := range map[string]func(Model) Model{
		"nothing typed":                 func(m Model) Model { return m },
		"the same labels spelled apart": func(m Model) Model { return basicsSet(m, 2, "backend , urgent") },
		"the same summary with spaces":  func(m Model) Model { return basicsSet(m, 0, "  Old summary ") },
	} {
		t.Run(name, func(t *testing.T) {
			j := newBasicsJira()
			m := edit(openBasics(t, j))
			m, cmd := basicsPress(m, "ctrl+s")
			if cmd != nil || m.view != viewDetail || j.puts.Load() != 0 || j.gets.Load() != 0 {
				t.Fatalf("cmd=%v view=%v puts=%d gets=%d", cmd != nil, m.view, j.puts.Load(), j.gets.Load())
			}
		})
	}
}

func TestBasicsRejectsInvalidInputInlineWithoutAnyRequest(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		edit       func(Model) Model
	}{
		{"summary emptied", "Summary cannot be empty", func(m Model) Model { return basicsSet(m, 0, "") }},
		{"summary only spaces", "Summary cannot be empty", func(m Model) Model { return basicsSet(m, 0, "   ") }},
		{"priority emptied", "Priority cannot be empty", func(m Model) Model { return basicsSet(m, 1, "") }},
		{"labels cleared", "Labels cannot be cleared", func(m Model) Model { return basicsSet(m, 2, "") }},
		{"empty label entry", "labels contain an empty entry", func(m Model) Model { return basicsSet(m, 2, "a,,b") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := newBasicsJira()
			m := tc.edit(openBasics(t, j))
			m, cmd := basicsPress(m, "ctrl+s")
			content := m.View().Content
			if cmd != nil || m.view != viewBasicsEdit || j.puts.Load() != 0 || j.gets.Load() != 0 {
				t.Fatalf("cmd=%v view=%v puts=%d gets=%d", cmd != nil, m.view, j.puts.Load(), j.gets.Load())
			}
			if !strings.Contains(content, tc.want) || !strings.Contains(content, "Edit summary, priority and labels") {
				t.Fatalf("missing inline error %q in an open form:\n%s", tc.want, content)
			}
		})
	}
}

func TestBasicsFailedUpdateKeepsTheFormAndTheTextAndSendsOneRequestPerSave(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			j := newBasicsJira()
			j.putStatus = status
			m := basicsSet(openBasics(t, j), 0, "New summary")
			m, cmd := basicsPress(m, "ctrl+s")
			m = basicsDeliver(m, cmd)
			content := m.View().Content
			if m.view != viewBasicsEdit || !strings.Contains(content, "Error") || !strings.Contains(content, "New summary") || j.puts.Load() != 1 || j.gets.Load() != 0 {
				t.Fatalf("view=%v puts=%d gets=%d:\n%s", m.view, j.puts.Load(), j.gets.Load(), content)
			}
			// A deliberate second Ctrl+S is a second request; nothing retries by itself.
			m, cmd = basicsPress(m, "ctrl+s")
			m = basicsDeliver(m, cmd)
			if j.puts.Load() != 2 {
				t.Fatalf("puts=%d, want exactly one request per Ctrl+S", j.puts.Load())
			}
		})
	}
}

func TestBasicsBusyFormStartsNothingElse(t *testing.T) {
	j := newBasicsJira()
	m := basicsSet(openBasics(t, j), 0, "New summary")
	m, cmd := basicsPress(m, "ctrl+s")
	if cmd == nil || !strings.Contains(m.View().Content, "Updating") {
		t.Fatalf("expected a pending save marked as updating:\n%s", m.View().Content)
	}
	for _, key := range []string{"ctrl+s", "tab", "x", "backspace"} {
		var again tea.Cmd
		m, again = basicsPress(m, key)
		if again != nil {
			t.Fatalf("%s started something while a save was in flight", key)
		}
	}
	m = basicsDeliver(m, cmd)
	if j.puts.Load() != 1 || j.gets.Load() != 1 || j.sentBody() != `{"fields":{"summary":"New summary"}}` || m.view != viewDetail {
		t.Fatalf("puts=%d gets=%d body=%s view=%v", j.puts.Load(), j.gets.Load(), j.sentBody(), m.view)
	}
}

func TestBasicsLateRepliesAreDiscarded(t *testing.T) {
	t.Run("after Esc", func(t *testing.T) {
		j := newBasicsJira()
		m := basicsSet(openBasics(t, j), 0, "New summary")
		m, cmd := basicsPress(m, "ctrl+s")
		m, _ = basicsPress(m, "esc")
		m = basicsDeliver(m, cmd)
		if m.view != viewDetail || m.detail.Fields.Summary != "Old summary" || m.toast != "" || j.puts.Load() != 1 {
			t.Fatalf("view=%v summary=%q toast=%q puts=%d: a reply after Esc must change nothing", m.view, m.detail.Fields.Summary, m.toast, j.puts.Load())
		}
	})
	t.Run("a newly opened form is not closed by an old reply", func(t *testing.T) {
		j := newBasicsJira()
		m := basicsSet(openBasics(t, j), 0, "New summary")
		m, cmd := basicsPress(m, "ctrl+s")
		m, _ = basicsPress(m, "esc")
		m, _ = basicsPress(m, "U")
		m = basicsDeliver(m, cmd)
		if m.view != viewBasicsEdit || m.detail.Fields.Summary != "Old summary" {
			t.Fatalf("view=%v summary=%q: an old reply touched the new form", m.view, m.detail.Fields.Summary)
		}
	})
	t.Run("after another ticket was opened", func(t *testing.T) {
		j := newBasicsJira()
		m := basicsSet(openBasics(t, j), 0, "New summary")
		m, cmd := basicsPress(m, "ctrl+s")
		other := *basicsIssue()
		other.Key = "TEST-2"
		m.detail = &other
		m = basicsDeliver(m, cmd)
		if m.detail.Key != "TEST-2" || m.detail.Fields.Summary != "Old summary" || m.toast != "" {
			t.Fatalf("detail=%+v toast=%q: a reply for another ticket was applied", m.detail, m.toast)
		}
	})
	t.Run("after the board was reloaded", func(t *testing.T) {
		j := newBasicsJira()
		m := basicsSet(openBasics(t, j), 0, "New summary")
		m, cmd := basicsPress(m, "ctrl+s")
		m.loadedBoardRequest++
		m = basicsDeliver(m, cmd)
		if m.detail.Fields.Summary != "Old summary" || m.toast != "" {
			t.Fatalf("summary=%q toast=%q: a reply for a previous board was applied", m.detail.Fields.Summary, m.toast)
		}
	})
}

func TestBasicsAppliedUpdateWithFailedRefreshRetriesOnlyTheRefresh(t *testing.T) {
	j := newBasicsJira()
	// A 403 is not retried: the Jira client retries a 5xx read by design, which would blur the count.
	j.getStatus = http.StatusForbidden
	m := basicsSet(openBasics(t, j), 0, "New summary")
	m, cmd := basicsPress(m, "ctrl+s")
	m = basicsDeliver(m, cmd)
	content := m.View().Content
	if m.view != viewBasicsEdit || !strings.Contains(content, "Update applied") || j.puts.Load() != 1 || j.gets.Load() != 1 {
		t.Fatalf("view=%v puts=%d gets=%d:\n%s", m.view, j.puts.Load(), j.gets.Load(), content)
	}
	j.getStatus = http.StatusOK
	m, cmd = basicsPress(m, "ctrl+s")
	if cmd == nil {
		t.Fatal("Ctrl+S did not retry the refresh")
	}
	m = basicsDeliver(m, cmd)
	if j.puts.Load() != 1 || j.gets.Load() != 2 || m.view != viewDetail || m.detail.Fields.Summary != "New summary" {
		t.Fatalf("puts=%d gets=%d view=%v: the retry must refresh only", j.puts.Load(), j.gets.Load(), m.view)
	}
}

func TestBasicsEscAfterAnAppliedUpdateSaysThatTheDetailIsStale(t *testing.T) {
	j := newBasicsJira()
	// A 403 is not retried: the Jira client retries a 5xx read by design, which would blur the count.
	j.getStatus = http.StatusForbidden
	m := basicsSet(openBasics(t, j), 0, "New summary")
	m, cmd := basicsPress(m, "ctrl+s")
	m = basicsDeliver(m, cmd)
	m, _ = basicsPress(m, "esc")
	if m.view != viewDetail || m.toast != "Update applied; detail refresh failed" || j.puts.Load() != 1 || j.gets.Load() != 1 {
		t.Fatalf("view=%v toast=%q puts=%d gets=%d", m.view, m.toast, j.puts.Load(), j.gets.Load())
	}
}

// A reply is also bound to the search that was active when the form was opened.
func TestBasicsLateReplyAfterTheSearchChangedIsDiscarded(t *testing.T) {
	j := newBasicsJira()
	m, _ := basicsModel(t, viewSearch, j.handler(t))
	m, _ = basicsPress(m, "U")
	m = basicsSet(m, 0, "New summary")
	m, cmd := basicsPress(m, "ctrl+s")
	m.activeSearchRequest++
	m = basicsDeliver(m, cmd)
	if m.detail.Fields.Summary != "Old summary" || m.toast != "" || j.puts.Load() != 1 {
		t.Fatalf("summary=%q toast=%q puts=%d: a reply for a previous search was applied", m.detail.Fields.Summary, m.toast, j.puts.Load())
	}
}
