package tui

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func assignmentKey(m Model, key string) (Model, tea.Cmd) {
	var msg tea.KeyPressMsg
	switch key {
	case "enter":
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		msg = tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		msg = tea.KeyPressMsg{Code: []rune(key)[0], Text: key}
	}
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func assignmentModel(t *testing.T, handler http.HandlerFunc) Model {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	m := New(jira.NewClient(srv.URL, "test", "test"), "")
	m.view = viewKanban
	m.width, m.height = 160, 40
	m.boards = []jira.Board{{ID: 7}}
	m.loadedBoardRequest = 1
	m.columns = []column{{name: "To Do", issues: []jira.Issue{{Key: "TEST-1"}}}}
	return m
}

func TestAssignmentPrompt(t *testing.T) {
	for _, value := range []string{"", "   "} {
		t.Run(fmt.Sprintf("empty_%q", value), func(t *testing.T) {
			calls := 0
			m := assignmentModel(t, func(w http.ResponseWriter, r *http.Request) { calls++; _, _ = fmt.Fprint(w, `{}`) })
			m, _ = assignmentKey(m, "A")
			if !strings.Contains(m.View().Content, "Assignee email") {
				t.Fatal("A did not open the assignee email prompt")
			}
			for range value {
				m, _ = assignmentKey(m, " ")
			}
			m, cmd := assignmentKey(m, "enter")
			if cmd != nil {
				t.Fatal("empty input submitted")
			}
			if !strings.Contains(m.View().Content, "Email is required") {
				t.Fatal("missing inline validation")
			}
			m, _ = assignmentKey(m, "esc")
			if m.view != viewKanban || calls != 0 {
				t.Fatalf("cancel view=%v requests=%d", m.view, calls)
			}
		})
	}
}

func assignmentEmail(m Model) Model {
	for _, r := range " person@example.com " {
		m, _ = assignmentKey(m, string(r))
	}
	return m
}

func TestAssignmentRequests(t *testing.T) {
	for _, scenario := range []string{"success", "unknown", "ambiguous", "lookup error", "assign error"} {
		t.Run(scenario, func(t *testing.T) {
			var requests []string
			m := assignmentModel(t, func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.Path)
				switch r.URL.Path {
				case "/rest/api/2/user/picker":
					if r.URL.Query().Get("query") != "person@example.com" {
						t.Errorf("query=%s", r.URL.RawQuery)
					}
					switch scenario {
					case "unknown":
						_, _ = fmt.Fprint(w, `{"users":[]}`)
					case "ambiguous":
						_, _ = fmt.Fprint(w, `{"users":[{"accountId":"wrong1"},{"accountId":"wrong2"}]}`)
					case "lookup error":
						w.WriteHeader(http.StatusForbidden)
					default:
						_, _ = fmt.Fprint(w, `{"users":[{"accountId":"account"}]}`)
					}
				case "/rest/api/3/user":
					if scenario == "ambiguous" {
						_, _ = fmt.Fprint(w, `{"accountId":"wrong","emailAddress":"other@example.com"}`)
						return
					}
					if r.URL.Query().Get("accountId") != "account" {
						t.Errorf("account query=%s", r.URL.RawQuery)
					}
					_, _ = fmt.Fprint(w, `{"accountId":"account","emailAddress":"person@example.com","displayName":"Pat Example"}`)
				case "/rest/api/3/search/jql":
					_, _ = fmt.Fprint(w, `{"issues":[]}`)
				case "/rest/api/3/issue/TEST-1/assignee":
					body, _ := io.ReadAll(r.Body)
					if r.Method != "PUT" || strings.TrimSpace(string(body)) != `{"accountId":"account"}` {
						t.Errorf("mutation=%s %s", r.Method, body)
					}
					if scenario == "assign error" {
						w.WriteHeader(http.StatusForbidden)
					} else {
						w.WriteHeader(http.StatusNoContent)
					}
				default:
					t.Errorf("unexpected request %s", r.URL)
				}
			})
			m, _ = assignmentKey(m, "A")
			m = assignmentEmail(m)
			m, cmd := assignmentKey(m, "enter")
			if cmd == nil {
				t.Fatal("valid submission did not resolve identity")
			}
			next, assign := m.Update(cmd())
			m = next.(Model)
			failedLookup := scenario == "unknown" || scenario == "ambiguous" || scenario == "lookup error"
			if failedLookup {
				if assign != nil {
					t.Fatal("failed identity scheduled mutation")
				}
				m, retry := assignmentKey(m, "enter")
				if retry != nil {
					t.Fatal("failed lookup retried without reopening")
				}
				if !strings.Contains(m.View().Content, "no user found") {
					t.Fatal("lookup error not visible")
				}
				for _, r := range requests {
					if strings.Contains(r, "/assignee") {
						t.Fatal("failed identity assigned")
					}
				}
				return
			}
			if assign == nil {
				t.Fatal("resolved identity did not schedule assign")
			}
			next, refresh := m.Update(assign())
			m = next.(Model)
			want := []string{"GET /rest/api/2/user/picker", "GET /rest/api/3/user", "PUT /rest/api/3/issue/TEST-1/assignee"}
			if !reflect.DeepEqual(requests, want) {
				t.Fatalf("requests=%v", requests)
			}
			if scenario == "assign error" {
				if m.err == nil || !strings.Contains(m.View().Content, "403") || refresh != nil {
					t.Fatalf("failed mutation err=%v refresh=%v", m.err, refresh != nil)
				}
				_, retry := assignmentKey(m, "enter")
				if retry != nil {
					t.Fatal("failed mutation retried")
				}
			} else if m.view != viewKanban || !strings.Contains(m.toast, "Pat Example") || refresh == nil || m.activeBoardRequest == 0 {
				t.Fatalf("success view=%v toast=%q refresh=%v", m.view, m.toast, refresh != nil)
			}
		})
	}
}

func TestAssignmentDetailAndHints(t *testing.T) {
	for _, origin := range []view{viewKanban, viewSearch} {
		t.Run(fmt.Sprint(origin), func(t *testing.T) {
			m := assignmentModel(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("unexpected request %s", r.URL) })
			m.view = viewDetail
			m.detailReturn = origin
			m.detail = &jira.Issue{Key: "TEST-1"}
			if !strings.Contains(m.View().Content, "[A] Assign") {
				t.Fatal("detail footer missing assignment hint")
			}
			m, _ = assignmentKey(m, "A")
			if !strings.Contains(m.View().Content, "Assignee email") {
				t.Fatal("detail assignment did not open")
			}
			m, _ = assignmentKey(m, "esc")
			if m.view != viewDetail || m.detail == nil {
				t.Fatal("cancel lost detail")
			}
		})
	}
	m := assignmentModel(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("unexpected request %s", r.URL) })
	m.width = 400
	if !strings.Contains(m.View().Content, "[A] Assign") {
		t.Fatal("board footer missing assignment hint")
	}
	m, _ = assignmentKey(m, "?")
	for _, text := range []string{"Assign ticket by email", "Add ticket to active sprint", "Transition ticket", "Permanently delete"} {
		if !strings.Contains(m.View().Content, text) {
			t.Fatalf("help missing %q", text)
		}
	}
	m, _ = assignmentKey(m, "esc")
	if m.helpOpen {
		t.Fatal("help did not close")
	}
	m.width = 15
	if strings.Contains(m.View().Content, "[A] Assign") || !strings.Contains(m.View().Content, "[?] Help") {
		t.Fatal("narrow footer did not collapse")
	}
}

func TestAssignmentSearchDetailRefresh(t *testing.T) {
	m := assignmentModel(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/user/picker":
			_, _ = fmt.Fprint(w, `{"users":[{"accountId":"account"}]}`)
		case "/rest/api/3/user":
			_, _ = fmt.Fprint(w, `{"accountId":"account","emailAddress":"person@example.com","displayName":"Pat Example"}`)
		case "/rest/api/3/issue/TEST-1/assignee":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s", r.URL)
		}
	})
	m.view = viewDetail
	m.detailReturn = viewSearch
	m.detail = &jira.Issue{Key: "TEST-1"}
	m.boards = nil
	m.loadedBoardRequest = 0
	m.activeSearchRequest = 8
	m.searchQuery = "project = TEST"
	m, _ = assignmentKey(m, "A")
	m = assignmentEmail(m)
	m, resolve := assignmentKey(m, "enter")
	next, assign := m.Update(resolve())
	m = next.(Model)
	next, refresh := m.Update(assign())
	m = next.(Model)
	if m.view != viewSearch || refresh == nil || !m.searchLoading || m.activeSearchRequest == 8 || m.detail != nil || m.toast != "Assigned TEST-1 to Pat Example" {
		t.Fatalf("search refresh view=%v toast=%q loading=%v", m.view, m.toast, m.searchLoading)
	}
}

func TestAssignmentExistingSprintShortcut(t *testing.T) {
	calls := 0
	m := assignmentModel(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.Path != "/rest/agile/1.0/sprint/11/issue" || strings.TrimSpace(string(body)) != `{"issues":["TEST-1"]}` {
			t.Errorf("request=%s %s %s", r.Method, r.URL, body)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	m.sprintID, m.sprintName = 11, "Current"
	m, cmd := assignmentKey(m, "a")
	if cmd == nil || m.view != viewKanban {
		t.Fatal("a no longer adds to sprint")
	}
	next, refresh := m.Update(cmd())
	m = next.(Model)
	if calls != 1 || m.toast != "Added TEST-1 to Current" || refresh == nil {
		t.Fatalf("calls=%d toast=%q", calls, m.toast)
	}
}

func TestAssignmentBoundedPaste(t *testing.T) {
	m := assignmentModel(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("unexpected %s", r.URL) })
	m, _ = assignmentKey(m, "A")
	next, _ := m.Update(tea.PasteMsg{Content: strings.Repeat("x", 300)})
	m = next.(Model)
	if len(m.assignment.input.Value()) != 254 {
		t.Fatalf("pasted input length=%d want=254", len(m.assignment.input.Value()))
	}
}

func TestAssignmentDuplicateReply(t *testing.T) {
	calls := 0
	m := assignmentModel(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "picker") {
			_, _ = fmt.Fprint(w, `{"users":[{"accountId":"account"}]}`)
		} else if strings.HasSuffix(r.URL.Path, "/user") {
			_, _ = fmt.Fprint(w, `{"accountId":"account","emailAddress":"person@example.com","displayName":"Pat Example"}`)
		} else {
			calls++
			w.WriteHeader(http.StatusNoContent)
		}
	})
	m, _ = assignmentKey(m, "A")
	m = assignmentEmail(m)
	m, cmd := assignmentKey(m, "enter")
	reply := cmd()
	next, assign := m.Update(reply)
	m = next.(Model)
	next, duplicate := m.Update(reply)
	m = next.(Model)
	if duplicate != nil {
		t.Fatal("duplicate resolve reply scheduled another assign")
	}
	done := assign()
	next, _ = m.Update(done)
	m = next.(Model)
	before := m
	next, duplicate = m.Update(done)
	m = next.(Model)
	if duplicate != nil || calls != 1 || !reflect.DeepEqual(before, m) {
		t.Fatal("duplicate done reply changed state")
	}
}

func TestAssignmentBusyAndLateReplies(t *testing.T) {
	for _, stage := range []string{"resolve", "assign"} {
		for _, exit := range []string{"duplicate", "escape", "board", "ticket", "search"} {
			t.Run(stage+"/"+exit, func(t *testing.T) {
				assigned := 0
				m := assignmentModel(t, func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/rest/api/2/user/picker":
						_, _ = fmt.Fprint(w, `{"users":[{"accountId":"account"}]}`)
					case "/rest/api/3/user":
						_, _ = fmt.Fprint(w, `{"accountId":"account","emailAddress":"person@example.com","displayName":"Pat Example"}`)
					case "/rest/api/3/issue/TEST-1/assignee":
						assigned++
						w.WriteHeader(http.StatusNoContent)
					default:
						t.Errorf("unexpected %s", r.URL)
					}
				})
				m, _ = assignmentKey(m, "A")
				m = assignmentEmail(m)
				m, cmd := assignmentKey(m, "enter")
				if cmd == nil {
					t.Fatal("submission not started")
				}
				if stage == "assign" {
					next, c := m.Update(cmd())
					m, cmd = next.(Model), c
					if cmd == nil {
						t.Fatal("assign not scheduled")
					}
				}
				switch exit {
				case "duplicate":
					for range 3 {
						var duplicate tea.Cmd
						m, duplicate = assignmentKey(m, "enter")
						if duplicate != nil {
							t.Fatal("duplicate submission scheduled")
						}
					}
					m, _ = assignmentKey(m, "x")
				case "escape":
					m, _ = assignmentKey(m, "esc")
				case "board":
					m.loadedBoardRequest++
				case "ticket":
					m.activeDetailRequest++
				case "search":
					m.activeSearchRequest++
				}
				before := m
				next, follow := m.Update(cmd())
				m = next.(Model)
				if exit != "duplicate" {
					if !reflect.DeepEqual(before, m) || follow != nil {
						t.Fatal("late reply changed new context")
					}
					want := 0
					if stage == "assign" {
						want = 1
					}
					if assigned != want {
						t.Fatalf("assign calls=%d want=%d", assigned, want)
					}
					return
				}
				if stage == "resolve" {
					if follow == nil {
						t.Fatal("missing assign")
					}
					next, _ = m.Update(follow())
					m = next.(Model)
				}
				if assigned != 1 || !strings.Contains(m.toast, "Pat Example") {
					t.Fatalf("calls=%d toast=%q", assigned, m.toast)
				}
			})
		}
	}
}
