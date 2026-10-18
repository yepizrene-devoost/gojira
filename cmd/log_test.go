package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestLogShowADFCommentCompatibility(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/issue/A-1/worklog" {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		_, _ = fmt.Fprint(w, `{"total":1,"worklogs":[{"author":{"displayName":"Ada"},"timeSpent":"1h","comment":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Worked"}]}]}}]}`)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--show", "--json")
	if code != 0 || stderr != "" || calls != 1 {
		t.Fatalf("code=%d calls=%d stderr=%s", code, calls, stderr)
	}
	var logs []jira.Worklog
	if err := json.Unmarshal([]byte(stdout), &logs); err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].Comment != "Worked" || logs[0].TimeSpent != "1h" {
		t.Fatal(stdout)
	}
}
