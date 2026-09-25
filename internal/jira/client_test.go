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
			fmt.Fprint(w, `{"startAt":0,"maxResults":2,"total":3,"isLast":false,
				"issues":[{"id":"1","key":"A-1"},{"id":"2","key":"A-2"}]}`)
		default:
			fmt.Fprint(w, `{"startAt":2,"maxResults":2,"total":3,"isLast":true,
				"issues":[{"id":"3","key":"A-3"}]}`)
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
