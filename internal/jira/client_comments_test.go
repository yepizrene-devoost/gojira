package jira

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func commentJSONBody(id, text string) string {
	return `{"id":"` + id + `","author":{"displayName":"Ada Lovelace","accountId":"acc-1"},"created":"2026-11-09T10:00:00.000+0000","updated":"2026-11-09T11:00:00.000+0000","body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"` + text + `"}]}]}}`
}

// ListComments must follow the page chain (startAt/maxResults/total/isLast)
// until every comment is collected, and must decode each comment's stable id.
func TestListCommentsPaginates(t *testing.T) {
	requestURIs := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURIs = append(requestURIs, r.URL.RequestURI())
		if !strings.HasPrefix(r.URL.Path, "/rest/api/3/issue/MON-1/comment") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		switch r.URL.Query().Get("startAt") {
		case "", "0":
			_, _ = io.WriteString(w, `{"startAt":0,"maxResults":2,"total":3,"isLast":false,"comments":[`+
				commentJSONBody("10001", "first")+`,`+commentJSONBody("10002", "second")+`]}`)
		case "2":
			_, _ = io.WriteString(w, `{"startAt":2,"maxResults":2,"total":3,"isLast":true,"comments":[`+
				commentJSONBody("10003", "third")+`]}`)
		default:
			t.Errorf("unexpected startAt %q", r.URL.Query().Get("startAt"))
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "email", "token")
	comments, total, err := client.ListComments("MON-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if total != 3 {
		t.Fatalf("total=%d", total)
	}
	if len(comments) != 3 || comments[0].ID != "10001" || comments[1].ID != "10002" || comments[2].ID != "10003" {
		t.Fatalf("comments=%+v", comments)
	}
	if comments[0].Author.DisplayName != "Ada Lovelace" {
		t.Fatalf("author=%q", comments[0].Author.DisplayName)
	}
	if got := comments[0].Body.Flatten(); got != "first" {
		t.Fatalf("body not decoded: %q", got)
	}
}

// A malformed page chain (startAt going backwards) must abort instead of
// looping forever.
func TestListCommentsRejectsNonMonotonicPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"startAt":0,"maxResults":2,"total":3,"isLast":false,"comments":[`+
			commentJSONBody("10001", "first")+`]}`)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "email", "token")
	if _, _, err := client.ListComments("MON-1"); err == nil {
		t.Fatal("expected error for non-monotonic pagination")
	}
}

// GetComment must address one comment by id on the entry path, with both
// segments escaped.
func TestGetCommentUsesEntryPath(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		_, _ = io.WriteString(w, commentJSONBody("10001", "the body"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "email", "token")
	comment, err := client.GetComment("MON-1", "10001")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if method != http.MethodGet || path != "/rest/api/3/issue/MON-1/comment/10001" {
		t.Fatalf("request=%s %s", method, path)
	}
	if comment.ID != "10001" || comment.Body.Flatten() != "the body" {
		t.Fatalf("comment=%+v", comment)
	}
}

// UpdateComment must PUT the new body as an ADF document to the entry path.
func TestUpdateCommentSendsADFBody(t *testing.T) {
	var method, path string
	var raw json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		raw = b
		_, _ = io.WriteString(w, commentJSONBody("10001", "edited"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "email", "token")
	if err := client.UpdateComment("MON-1", "10001", "edited text"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if method != http.MethodPut || path != "/rest/api/3/issue/MON-1/comment/10001" {
		t.Fatalf("request=%s %s", method, path)
	}
	var body struct {
		Body ADFDoc `json:"body"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("body=%s err=%v", raw, err)
	}
	if got := body.Body.Flatten(); got != "edited text" {
		t.Fatalf("sent body=%q", got)
	}
}

// DeleteComment must issue a body-less DELETE on the entry path.
func TestDeleteCommentUsesEntryPath(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "email", "token")
	if err := client.DeleteComment("MON-1", "10001"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if method != http.MethodDelete || path != "/rest/api/3/issue/MON-1/comment/10001" {
		t.Fatalf("request=%s %s", method, path)
	}
}

// Path helpers must escape issue keys and comment ids so a crafted id can
// never change the path shape.
func TestCommentEntryPathEscapesSegments(t *testing.T) {
	got := commentEntryPath("MO N-1", "10/001")
	want := "/rest/api/3/issue/MO%20N-1/comment/10%2F001"
	if got != want {
		t.Fatalf("path=%q want=%q", got, want)
	}
}

// Permission failures must surface as classified API errors, not silent empty
// results.
func TestListCommentsReportsForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"errorMessages":["You do not have permission to view comments"],"errors":{}}`)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "email", "token")
	_, _, err := client.ListComments("MON-1")
	if err == nil {
		t.Fatal("expected error for forbidden comment read")
	}
	if !strings.Contains(err.Error(), "permission") {
		t.Fatalf("err=%v", err)
	}
}
