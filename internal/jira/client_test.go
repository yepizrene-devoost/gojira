package jira

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// GetBoardIssues must page through the Agile endpoint: a single request is
// capped server-side and silently truncates large sprints (the bug that hid
// a freshly created ticket beyond the first page).
func TestGetBoardIssuesPaginates(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("startAt") {
		case "0":
			if _, err := fmt.Fprint(w, `{"startAt":0,"maxResults":2,"total":3,"isLast":false,
				"issues":[{"id":"1","key":"A-1"},{"id":"2","key":"A-2"}]}`); err != nil {
				t.Errorf("write page 1: %v", err)
			}
		default:
			if _, err := fmt.Fprint(w, `{"startAt":2,"maxResults":2,"total":3,"isLast":true,
				"issues":[{"id":"3","key":"A-3"}]}`); err != nil {
				t.Errorf("write page 2: %v", err)
			}
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "e@x.com", "tok")
	issues, err := client.GetBoardIssues(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 3 {
		t.Fatalf("got %d issues, want 3 (pagination stopped early?)", len(issues))
	}
	if issues[2].Key != "A-3" {
		t.Fatalf("last issue = %s, want A-3", issues[2].Key)
	}
	if hits != 2 {
		t.Fatalf("made %d requests, want 2", hits)
	}
}

func TestSearchJQLPreservesQueryAtServerBoundary(t *testing.T) {
	const wantJQL = `project = "A&B" AND summary ~ "50% + #tag"`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("jql"); got != wantJQL {
			t.Errorf("JQL query = %q, want %q", got, wantJQL)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprint(w, `{"issues":[],"total":0,"maxResults":50}`); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "e@x.com", "tok")
	if _, _, err := client.SearchJQL(wantJQL, 0); err != nil {
		t.Fatal(err)
	}
}

// A 2xx response whose body does not decode must surface as an error: every
// decode path returns the json.Unmarshal failure instead of handing back zero
// values that read like an empty result (the contract fixed in 4f58abc).
func TestMalformedSuccessBodySurfacesDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Truncated JSON: syntactically invalid for every target type.
		if _, err := fmt.Fprint(w, `{"issues": [}`); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "e@x.com", "tok")

	tests := []struct {
		name string
		call func() error
	}{
		{"TestConnection", func() error { _, err := client.TestConnection(); return err }},
		{"GetBoards", func() error { _, err := client.GetBoards(); return err }},
		{"GetBoardConfig", func() error { _, err := client.GetBoardConfig(1); return err }},
		{"GetSprints", func() error { _, err := client.GetSprints(1); return err }},
		{"GetBoardIssues", func() error { _, err := client.GetBoardIssues(1, 0); return err }},
		{"GetIssue", func() error { _, err := client.GetIssue("A-1"); return err }},
		{"GetIssueFull", func() error { _, err := client.GetIssueFull("A-1"); return err }},
		{"GetProjects", func() error { _, err := client.GetProjects(); return err }},
		{"SearchJQL", func() error { _, _, err := client.SearchJQL("project = A", 10); return err }},
		{"GetTransitions", func() error { _, err := client.GetTransitions("A-1"); return err }},
		{"GetWorklog", func() error { _, err := client.GetWorklog("A-1"); return err }},
		{"CreateIssue", func() error {
			_, err := client.CreateIssue("A", "Task", "summary", nil)
			return err
		}},
		{"GetIssueTypes", func() error { _, err := client.GetIssueTypes("A"); return err }},
		{"userPicker", func() error { _, err := client.userPicker("rene@devoost.com"); return err }},
		{"getUser", func() error { _, err := client.getUser("712020:7e4261c7"); return err }},
		// Both fallbacks fail here, so the resolver must say so rather than
		// report a plain "no user found".
		{"ResolveAccountID", func() error {
			_, _, err := client.ResolveAccountID("rene@devoost.com")
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatal("malformed 2xx body was accepted; want a decode error")
			}
		})
	}
}

// A well-formed body of the wrong shape is a decode error too, so the contract
// does not depend on the body being syntactically broken.
func TestWrongShapeSuccessBodySurfacesDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Valid JSON, but an array where the endpoint promises an object.
		if _, err := fmt.Fprint(w, `[]`); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "e@x.com", "tok")
	if _, err := client.GetIssue("A-1"); err == nil {
		t.Fatal("wrong-shape 2xx body was accepted; want a decode error")
	}
}

// ResolveAccountID must go picker → exact email verification → JQL fallback,
// because /user/username is gone and users/search no longer exposes emails.
func TestResolveAccountID(t *testing.T) {
	const email = "rene@devoost.com"
	const reneID = "712020:7e4261c7"
	const otherID = "557058:other"

	tests := []struct {
		name string
		// picker: JSON for the picker response; "" → picker 404s
		picker string
		// emails per accountId for the /user?accountId verification step
		emails map[string]string
		// jql: JSON for the search/jql response; "" → endpoint 404s
		jql string
		wantID   string
		wantErr  bool
	}{
		{
			name:   "picker sole candidate verified by email",
			picker: `{"users":[{"accountId":"` + reneID + `","displayName":"René"}],"total":1}`,
			emails: map[string]string{reneID: email},
			wantID: reneID,
		},
		{
			name:   "picker fuzzy match rejected, second candidate wins",
			picker: `{"users":[{"accountId":"` + otherID + `","displayName":"Other"},{"accountId":"` + reneID + `","displayName":"René"}],"total":2}`,
			emails: map[string]string{otherID: "other@x.com", reneID: email},
			wantID: reneID,
		},
		{
			name:   "emails hidden from API: sole picker match accepted",
			picker: `{"users":[{"accountId":"` + reneID + `","displayName":"René"}],"total":1}`,
			emails: map[string]string{reneID: ""},
			wantID: reneID,
		},
		{
			name:   "picker empty falls back to JQL",
			picker: `{"users":[],"total":0}`,
			emails: map[string]string{},
			jql:    `{"issues":[{"fields":{"creator":{"accountId":"` + reneID + `","displayName":"René","emailAddress":"RENE@devoost.com"}}}]}`,
			wantID: reneID,
		},
		{
			name:   "picker down falls back to JQL",
			picker: "",
			jql:    `{"issues":[{"fields":{"assignee":{"accountId":"` + reneID + `","displayName":"René","emailAddress":"` + email + `"}}}]}`,
			wantID: reneID,
		},
		{
			name:   "no path resolves the email",
			picker: `{"users":[],"total":0}`,
			jql:    `{"issues":[]}`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/rest/api/2/user/picker":
					if tc.picker == "" {
						w.WriteHeader(404)
						return
					}
					if _, err := fmt.Fprint(w, tc.picker); err != nil {
						t.Errorf("write picker response: %v", err)
					}
				case "/rest/api/3/user":
					e, known := tc.emails[r.URL.Query().Get("accountId")]
					if !known {
						w.WriteHeader(404)
						return
					}
					if _, err := fmt.Fprintf(w, `{"accountId":%q,"displayName":"René","emailAddress":%q}`,
						r.URL.Query().Get("accountId"), e); err != nil {
						t.Errorf("write user response: %v", err)
					}
				case "/rest/api/3/search/jql":
					if tc.jql == "" {
						w.WriteHeader(404)
						return
					}
					if _, err := fmt.Fprint(w, tc.jql); err != nil {
						t.Errorf("write jql response: %v", err)
					}
				default:
					w.WriteHeader(404)
				}
			}))
			defer srv.Close()

			client := NewClient(srv.URL, "e@x.com", "tok")
			id, _, err := client.ResolveAccountID(email)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id %q", id)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if id != tc.wantID {
				t.Fatalf("got %q, want %q", id, tc.wantID)
			}
		})
	}
}
