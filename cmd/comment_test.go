package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func commentStubJSON(id, author, text string) string {
	return `{"id":"` + id + `","author":{"displayName":"` + author + `"},"created":"2026-11-09T10:00:00.000+0000","updated":"2026-11-09T11:00:00.000+0000","body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"` + text + `"}]}]}}`
}

// ─── S1/S2/D2: comment listing ───────────────────────────────────────────

// `comment KEY --list` walks the paginated endpoint and prints every comment
// with its id, author and timestamp on stdout.
func TestCommentListPrintsAllComments(t *testing.T) {
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		if r.URL.Path != "/rest/api/3/issue/MON-1/comment" {
			t.Errorf("path=%q", r.URL.Path)
		}
		switch r.URL.Query().Get("startAt") {
		case "", "0":
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":2,"total":3,"isLast":false,"comments":[
				`+commentStubJSON("10001", "Ada Lovelace", "first")+`,
				`+commentStubJSON("10002", "Grace Hopper", "second")+`]}`)
		case "2":
			_, _ = fmt.Fprint(w, `{"startAt":2,"maxResults":2,"total":3,"isLast":true,"comments":[
				`+commentStubJSON("10003", "Alan Turing", "third")+`]}`)
		default:
			t.Errorf("unexpected startAt=%q", r.URL.Query().Get("startAt"))
		}
	}))
	defer srv.Close()

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "comment", "MON-1", "--list")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
	for _, want := range []string{"10001", "Ada Lovelace", "10002", "Grace Hopper", "10003", "Alan Turing", "third"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q: %q", want, stdout)
		}
	}
}

// `comment KEY --list --json` writes one JSON document with total and every
// comment id.
func TestCommentListJSONHasTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":50,"total":1,"isLast":true,"comments":[
			`+commentStubJSON("10001", "Ada Lovelace", "first")+`]}`)
	}))
	defer srv.Close()

	code, stdout, _ := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "comment", "MON-1", "--list", "--json")
	if code != 0 {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if !strings.Contains(stdout, `"total": 1`) || !strings.Contains(stdout, `"id": "10001"`) {
		t.Fatalf("json=%q", stdout)
	}
}

// ─── S2/D2: read one comment by id ───────────────────────────────────────

func TestCommentReadByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/MON-1/comment/10007" {
			t.Errorf("path=%q", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, commentStubJSON("10007", "Ada Lovelace", "the body"))
	}))
	defer srv.Close()

	code, stdout, _ := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "comment", "MON-1", "--id", "10007")
	if code != 0 {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if !strings.Contains(stdout, "Ada Lovelace") || !strings.Contains(stdout, "the body") {
		t.Fatalf("stdout=%q", stdout)
	}
}

// A comment id that Jira does not know surfaces as a failure with a
// non-zero exit code; in JSON mode the v1 envelope carries the code.
func TestCommentReadUnknownIDFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errorMessages":["Comment with id '404' does not exist"],"errors":{}}`)
	}))
	defer srv.Close()

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "comment", "MON-1", "--id", "404", "--json")
	if code == 0 {
		t.Fatalf("expected failure, stdout=%q", stdout)
	}
	if !strings.Contains(stderr, `"code": `) {
		t.Fatalf("stderr=%q", stderr)
	}
}

// ─── S1/D2: edit a comment by id ─────────────────────────────────────────

// `comment KEY --id ID --text "..."` PUTs the replacement body as ADF and
// warns on stderr that the rich body is replaced.
func TestCommentEditSendsADFAndWarns(t *testing.T) {
	var method, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		_, _ = fmt.Fprint(w, commentStubJSON("10007", "Ada Lovelace", "edited text"))
	}))
	defer srv.Close()

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "comment", "MON-1", "--id", "10007", "--text", "edited text")
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if method != http.MethodPut {
		t.Fatalf("method=%q body=%q", method, body)
	}
	if !strings.Contains(body, `"type":"doc"`) || !strings.Contains(body, "edited text") {
		t.Fatalf("body is not the ADF replacement: %q", body)
	}
	if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "replaces") {
		t.Fatalf("expected ADF replacement warning on stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "warning") {
		t.Fatalf("warning leaked to stdout: %q", stdout)
	}
}

// Editing requires the replacement text: `--id` with `--text` edits, `--id`
// alone reads. A `--text` without `--id` is rejected before any HTTP request
// (covered by TestCommentFlagValidationBeforeRequests).
func TestCommentReadByIDJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, commentStubJSON("10007", "Ada Lovelace", "the body"))
	}))
	defer srv.Close()

	code, stdout, _ := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "comment", "MON-1", "--id", "10007", "--json")
	if code != 0 {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if !strings.Contains(stdout, `"id": "10007"`) || !strings.Contains(stdout, `"body": "the body"`) {
		t.Fatalf("json=%q", stdout)
	}
}

// ─── S4/D2: delete is destructive and needs the echoed id ────────────────

func TestCommentDeleteRequiresConfirm(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if code, _, _ := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "comment", "MON-1", "--id", "10007", "--delete"); code == 0 {
		t.Fatal("delete without --confirm must fail")
	}
	if code, _, _ := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "comment", "MON-1", "--id", "10007", "--delete", "--confirm", "99999"); code == 0 {
		t.Fatal("delete with mismatched --confirm must fail")
	}
	if requests != 0 {
		t.Fatalf("unexpected HTTP requests: %d", requests)
	}
}

func TestCommentDeleteSucceedsWithEchoedID(t *testing.T) {
	var method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	code, stdout, _ := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "comment", "MON-1", "--id", "10007", "--delete", "--confirm", "10007")
	if code != 0 {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if method != http.MethodDelete {
		t.Fatalf("method=%q", method)
	}
	if !strings.Contains(stdout, "10007") {
		t.Fatalf("stdout=%q", stdout)
	}
}

// ─── existing add-comment behavior is preserved ──────────────────────────

func TestCommentAddPreserved(t *testing.T) {
	var method, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		_, _ = fmt.Fprint(w, `{"id":"10008"}`)
	}))
	defer srv.Close()

	code, stdout, _ := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "comment", "MON-1", "hello world")
	if code != 0 {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if method != http.MethodPost || !strings.Contains(body, "hello world") {
		t.Fatalf("method=%q body=%q", method, body)
	}
}

// ─── flag validation before any request ──────────────────────────────────

func TestCommentFlagValidationBeforeRequests(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	cases := [][]string{
		{"comment", "MON-1", "--list", "--id", "10001"},
		{"comment", "MON-1", "--delete"},
		{"comment", "MON-1", "--text", "x"},
		{"comment", "MON-1", "--id", "1", "--confirm", "1"},
		{"comment", "MON-1", "--list", "--text", "x"},
	}
	for _, args := range cases {
		if code, _, _ := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), args...); code == 0 {
			t.Fatalf("%v: expected failure", args)
		}
	}
	if requests != 0 {
		t.Fatalf("unexpected HTTP requests: %d", requests)
	}
}
