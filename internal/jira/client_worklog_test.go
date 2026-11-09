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

// S1 — the read path must expose Jira's worklog id, the handle needed to
// update or delete one entry.
func TestGetWorklogExposesID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/issue/ABC-1/worklog" {
			t.Errorf("request=%s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"worklogs":[{"id":"123","author":{"displayName":"Ada"},"comment":"Worked","timeSpent":"1h","started":"2025-01-02T00:00:00Z"}]}`)
	}))
	defer srv.Close()

	logs, err := NewClient(srv.URL, "email", "token").GetWorklog("ABC-1")
	if err != nil || len(logs) != 1 {
		t.Fatalf("logs=%v err=%v", logs, err)
	}
	if logs[0].ID != "123" || logs[0].TimeSpent != "1h" || logs[0].Comment != "Worked" {
		t.Fatalf("worklog=%+v", logs[0])
	}
}

// S2 — a partial update sends only what the caller supplied, so the untouched
// field keeps its Jira value.
func TestUpdateWorklogSendsOnlySuppliedFields(t *testing.T) {
	cases := []struct {
		name      string
		timeSpent string
		comment   string
		wantField map[string]bool
	}{
		{"time only", "3h", "", map[string]bool{"timeSpent": true}},
		{"comment only", "", "Corrected", map[string]bool{"comment": true}},
		{"both", "3h", "Corrected", map[string]bool{"timeSpent": true, "comment": true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var method, path string
			var body map[string]json.RawMessage
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method, path = r.Method, r.URL.Path
				raw, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Errorf("body=%s err=%v", raw, err)
				}
				_, _ = io.WriteString(w, `{"id":"123"}`)
			}))
			defer srv.Close()

			if err := NewClient(srv.URL, "email", "token").UpdateWorklog("ABC-1", "123", tc.timeSpent, tc.comment); err != nil {
				t.Fatalf("err=%v", err)
			}
			if method != http.MethodPut || path != "/rest/api/3/issue/ABC-1/worklog/123" {
				t.Fatalf("request=%s %s", method, path)
			}
			for field := range map[string]bool{"timeSpent": true, "comment": true} {
				_, sent := body[field]
				if sent != tc.wantField[field] {
					t.Fatalf("%s sent=%v body=%v", field, sent, body)
				}
			}
			if tc.timeSpent != "" && string(body["timeSpent"]) != `"`+tc.timeSpent+`"` {
				t.Fatalf("timeSpent=%s", body["timeSpent"])
			}
			if tc.comment != "" {
				raw := body["comment"]
				if len(raw) == 0 || raw[0] == '"' {
					t.Fatalf("comment is not an ADF document: %s", raw)
				}
				var doc ADFDoc
				if err := json.Unmarshal(raw, &doc); err != nil {
					t.Fatal(err)
				}
				if doc.Flatten() != tc.comment {
					t.Fatalf("comment=%q", doc.Flatten())
				}
			}
		})
	}
}

// S2 — an update that changes nothing must fail locally instead of issuing a
// no-op request Jira could silently accept.
func TestUpdateWorklogRequiresAChange(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "email", "token").UpdateWorklog("ABC-1", "123", "", "")
	if err == nil || err.Error() != "update requires timeSpent or comment" {
		t.Fatalf("err=%v", err)
	}
	if calls != 0 {
		t.Fatalf("calls=%d", calls)
	}
}

// S3 — delete addresses one worklog entry and sends no body.
func TestDeleteWorklogSendsEntryPath(t *testing.T) {
	var method, path string
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL, "email", "token").DeleteWorklog("ABC-1", "123"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if method != http.MethodDelete || path != "/rest/api/3/issue/ABC-1/worklog/123" {
		t.Fatalf("request=%s %s", method, path)
	}
	if len(body) != 0 {
		t.Fatalf("body=%s", body)
	}
}

// S12 — after an update the client re-reads the entry and accepts the duration
// Jira kept, without issuing any mutation.
func TestVerifyWorklogUpdateAcceptsKeptDuration(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/issue/ABC-1/worklog" {
			t.Errorf("request=%s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"worklogs":[{"id":"123","timeSpent":"30m"},{"id":"456","timeSpent":"1h"}]}`)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL, "email", "token").VerifyWorklogUpdate("ABC-1", "123", "30m"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}

// S12 — a duration Jira did not keep is reported with both values, so the
// caller can warn instead of claiming success.
func TestVerifyWorklogUpdateReportsKeptDuration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"worklogs":[{"id":"123","timeSpent":"15m"}]}`)
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "email", "token").VerifyWorklogUpdate("ABC-1", "123", "30m")
	if err == nil || err.Error() != "Jira reports 15m for worklog 123 (requested 30m)" {
		t.Fatalf("err=%v", err)
	}
}

// S12 — an entry Jira no longer reports is a mismatch too.
func TestVerifyWorklogUpdateReportsMissingEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"worklogs":[]}`)
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "email", "token").VerifyWorklogUpdate("ABC-1", "123", "30m")
	if err == nil || err.Error() != "worklog 123 is not reported by Jira after the update" {
		t.Fatalf("err=%v", err)
	}
}

// S12 — a caller that changed no duration pays no extra read.
func TestVerifyWorklogUpdateSkipsBlankRequest(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))
	defer srv.Close()

	if err := NewClient(srv.URL, "email", "token").VerifyWorklogUpdate("ABC-1", "123", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if calls != 0 {
		t.Fatalf("calls=%d", calls)
	}
}

// S14 — clearing a comment is explicit: it sends a JSON null, never a blank
// string, so Jira cannot read it as "leave the comment alone".
func TestClearWorklogCommentSendsNull(t *testing.T) {
	var method, path string
	var body map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body=%s err=%v", raw, err)
		}
		_, _ = io.WriteString(w, `{"id":"123"}`)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL, "email", "token").ClearWorklogComment("ABC-1", "123"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if method != http.MethodPut || path != "/rest/api/3/issue/ABC-1/worklog/123" {
		t.Fatalf("request=%s %s", method, path)
	}
	if len(body) != 1 || string(body["comment"]) != "null" {
		t.Fatalf("body=%v", body)
	}
}

// S15 — the management list must read the full worklog collection instead of
// the page embedded in the ticket read.
func TestGetAllWorklogsReadsDedicatedEndpoint(t *testing.T) {
	var rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/ABC-1/worklog" {
			t.Errorf("path=%s", r.URL.Path)
		}
		rawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"startAt":0,"maxResults":1000,"total":2,"worklogs":[{"id":"123","timeSpent":"15m"},{"id":"456","timeSpent":"10m"}]}`)
	}))
	defer srv.Close()

	logs, err := NewClient(srv.URL, "email", "token").GetAllWorklogs("ABC-1")
	if err != nil || len(logs) != 2 || logs[1].ID != "456" {
		t.Fatalf("logs=%v err=%v", logs, err)
	}
	if rawQuery != "startAt=0&maxResults=1000" {
		t.Fatalf("query=%q", rawQuery)
	}
}

// S14 — changing the duration and clearing the comment must travel in one
// request, so a failure cannot leave the worklog half-edited.
func TestUpdateWorklogAndClearCommentSendsOneRequest(t *testing.T) {
	requests := 0
	var method, path string
	var body map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		method, path = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body=%s err=%v", raw, err)
		}
		_, _ = io.WriteString(w, `{"id":"123"}`)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL, "email", "token").UpdateWorklogAndClearComment("ABC-1", "123", "3h"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if requests != 1 {
		t.Fatalf("requests=%d", requests)
	}
	if method != http.MethodPut || path != "/rest/api/3/issue/ABC-1/worklog/123" {
		t.Fatalf("request=%s %s", method, path)
	}
	if string(body["timeSpent"]) != `"3h"` || string(body["comment"]) != "null" {
		t.Fatalf("body=%v", body)
	}
}
