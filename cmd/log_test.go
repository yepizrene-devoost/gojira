package cmd

import (
	"encoding/json"
	"fmt"
	"io"
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

// `log --comment` must still deliver the text the user typed, now as the ADF
// document Jira accepts instead of a plain string.
func TestLogCommentIsWrittenAsADF(t *testing.T) {
	var body map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/issue/A-1/worklog" {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body=%s err=%v", raw, err)
		}
		_, _ = fmt.Fprint(w, `{"id":"1"}`)
	}))
	defer srv.Close()
	code, _, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--time", "2h", "--comment", "Fixed auth bug")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if string(body["timeSpent"]) != `"2h"` {
		t.Fatalf("timeSpent=%s", body["timeSpent"])
	}
	raw, sent := body["comment"]
	if !sent || len(raw) == 0 || raw[0] == '"' {
		t.Fatalf("comment is not an ADF document: %s", raw)
	}
	var doc jira.ADFDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Flatten() != "Fixed auth bug" {
		t.Fatalf("comment=%q", doc.Flatten())
	}
}

// A worklog without `--comment` must not send an empty comment field: Jira
// rejects the whole request with "Worklog must not be null." when it does.
func TestLogWithoutCommentOmitsIt(t *testing.T) {
	var body map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body=%s err=%v", raw, err)
		}
		_, _ = fmt.Fprint(w, `{"id":"1"}`)
	}))
	defer srv.Close()
	code, _, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--time", "30m")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if string(body["timeSpent"]) != `"30m"` {
		t.Fatalf("timeSpent=%s", body["timeSpent"])
	}
	if _, sent := body["comment"]; sent {
		t.Fatalf("blank comment was sent: %v", body)
	}
}
