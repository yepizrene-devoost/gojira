package jira

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Jira Platform v3 rejects a worklog whose comment is an empty string
// ("Worklog body is not valid." / "Worklog must not be null."), so an empty
// comment must be omitted from the request body entirely.
func TestAddWorklogOmitsEmptyComment(t *testing.T) {
	var method, path string
	var body map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body=%s err=%v", raw, err)
		}
		_, _ = io.WriteString(w, `{"id":"1"}`)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL, "email", "token").AddWorklog("ABC-1", "2h", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if method != http.MethodPost || path != "/rest/api/3/issue/ABC-1/worklog" {
		t.Fatalf("request=%s %s", method, path)
	}
	if _, sent := body["comment"]; sent {
		t.Fatalf("empty comment was sent: %v", body)
	}
	if string(body["timeSpent"]) != `"2h"` {
		t.Fatalf("timeSpent=%s", body["timeSpent"])
	}
}

// A supplied comment must travel as an ADF document, matching AddComment's
// {"body": ADFDoc} convention for every Platform v3 text body.
func TestAddWorklogSendsCommentAsADF(t *testing.T) {
	var body map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body=%s err=%v", raw, err)
		}
		_, _ = io.WriteString(w, `{"id":"1"}`)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL, "email", "token").AddWorklog("ABC-1", "2h", "Reviewed the API"); err != nil {
		t.Fatalf("err=%v", err)
	}
	raw, sent := body["comment"]
	if !sent {
		t.Fatal("comment was not sent")
	}
	if len(raw) == 0 || raw[0] == '"' {
		t.Fatalf("comment is not an ADF document: %s", raw)
	}
	var doc ADFDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("comment=%s err=%v", raw, err)
	}
	if doc.Type != "doc" || doc.Version != 1 || doc.Flatten() != "Reviewed the API" {
		t.Fatalf("doc=%s flattened=%q", raw, doc.Flatten())
	}
	if string(body["timeSpent"]) != `"2h"` {
		t.Fatalf("timeSpent=%s", body["timeSpent"])
	}
}
