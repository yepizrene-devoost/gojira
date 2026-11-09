package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// S4: `log KEY --id ID --time DURATION` PUTs only the duration on the
// worklog entry path and prints the fixed success line. S12 adds one re-read
// of the entry afterwards; when Jira keeps the requested duration there is no
// warning.
func TestLogUpdateWithTimeOnly(t *testing.T) {
	var (
		method   string
		path     string
		body     map[string]json.RawMessage
		verifies int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/A-1/worklog" {
			verifies++
			_, _ = fmt.Fprint(w, `{"total":1,"worklogs":[{"id":"77","timeSpent":"3h"}]}`)
			return
		}
		method, path = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body=%s err=%v", raw, err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--id", "77", "--time", "3h")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if method != http.MethodPut || path != "/rest/api/3/issue/A-1/worklog/77" {
		t.Fatalf("request = %s %s", method, path)
	}
	if verifies != 1 {
		t.Fatalf("verification reads=%d, want the S12 re-read after the update", verifies)
	}
	if string(body["timeSpent"]) != `"3h"` {
		t.Fatalf("timeSpent=%s body=%v", body["timeSpent"], body)
	}
	if _, sent := body["comment"]; sent {
		t.Fatalf("comment must be omitted so Jira keeps it: %v", body)
	}
	if stdout != "✓ Updated worklog 77 on A-1\n" {
		t.Fatalf("stdout=%q", stdout)
	}
}

// S4: `--comment` alone updates the comment as an ADF document and omits the
// duration so Jira keeps the recorded time.
func TestLogUpdateWithCommentOnlySendsADF(t *testing.T) {
	var (
		method string
		path   string
		body   map[string]json.RawMessage
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body=%s err=%v", raw, err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--id", "77", "--comment", "Adjusted estimate")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if method != http.MethodPut || path != "/rest/api/3/issue/A-1/worklog/77" {
		t.Fatalf("request = %s %s", method, path)
	}
	if _, sent := body["timeSpent"]; sent {
		t.Fatalf("timeSpent must be omitted: %v", body)
	}
	raw, sent := body["comment"]
	if !sent || len(raw) == 0 || raw[0] == '"' {
		t.Fatalf("comment is not an ADF document: %s", raw)
	}
	var doc jira.ADFDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Flatten() != "Adjusted estimate" {
		t.Fatalf("comment=%q", doc.Flatten())
	}
	if stdout != "✓ Updated worklog 77 on A-1\n" {
		t.Fatalf("stdout=%q", stdout)
	}
}

// S12: when Jira keeps another duration, the accepted mutation still exits 0
// and the mismatch is a `warning:` line on stderr after the success line.
func TestLogUpdateWarnsWhenJiraKeepsAnotherDuration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/3/issue/A-1/worklog/77":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/A-1/worklog":
			_, _ = fmt.Fprint(w, `{"total":1,"worklogs":[{"id":"77","timeSpent":"15m"}]}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--id", "77", "--time", "30m")
	if code != 0 {
		t.Fatalf("code=%d, want 0: the mutation was accepted", code)
	}
	if stdout != "✓ Updated worklog 77 on A-1\n" {
		t.Fatalf("stdout=%q", stdout)
	}
	if stderr != "warning: jira reports 15m for worklog 77 (requested 30m)\n" {
		t.Fatalf("stderr=%q, want the warning on stderr only", stderr)
	}
}

// S12: with `--json` the stdout document stays valid JSON with no warning
// mixed in; the mismatch goes to stderr.
func TestLogUpdateJSONWarnsOnStderrOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/3/issue/A-1/worklog/77":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/A-1/worklog":
			_, _ = fmt.Fprint(w, `{"total":1,"worklogs":[{"id":"77","timeSpent":"15m"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/A-1":
			_, _ = fmt.Fprint(w, testIssueJSON)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--id", "77", "--time", "30m", "--json")
	if code != 0 {
		t.Fatalf("code=%d, want 0", code)
	}
	var result map[string]any
	assertSingleJSONDocument(t, []byte(stdout), &result)
	if result["schemaVersion"] != "v1" || result["key"] != "A-1" {
		t.Fatalf("stdout is not the mutation JSON: %v", result)
	}
	if stderr != "warning: jira reports 15m for worklog 77 (requested 30m)\n" {
		t.Fatalf("stderr=%q, want the warning on stderr only", stderr)
	}
}

// S4: `--id` with neither `--time` nor `--comment` is a validation error and
// must not touch the network.
func TestLogUpdateWithoutFieldsMakesNoRequest(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--id", "77")
	if code != 1 || stdout != "" {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if stderr != "Error: --time or --comment is required with --id\n" {
		t.Fatalf("stderr=%q", stderr)
	}
	if requests != 0 {
		t.Fatalf("requests=%d, want 0", requests)
	}
}

// S4: `--id --json` reuses the shared mutation JSON contract.
func TestLogUpdateJSONUsesMutationResult(t *testing.T) {
	mutationSeen := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/3/issue/A-1/worklog/77":
			mutationSeen = true
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/A-1/worklog":
			// S12 re-read: Jira kept 3h, so no warning may appear anywhere.
			_, _ = fmt.Fprint(w, `{"total":1,"worklogs":[{"id":"77","timeSpent":"3h"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/A-1":
			if !mutationSeen {
				t.Errorf("result fetched before mutation")
			}
			_, _ = fmt.Fprint(w, testIssueJSON)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--id", "77", "--time", "3h", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	var result map[string]any
	assertSingleJSONDocument(t, []byte(stdout), &result)
	if result["schemaVersion"] != "v1" || result["key"] != "A-1" {
		t.Fatalf("result=%#v, want TicketJSON v1 for A-1", result)
	}
}

// S5: a confirmed delete issues DELETE on the entry path with no body.
func TestLogDeleteWorklog(t *testing.T) {
	var (
		method string
		path   string
		body   []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--id", "77", "--delete", "--confirm", "77")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if method != http.MethodDelete || path != "/rest/api/3/issue/A-1/worklog/77" {
		t.Fatalf("request = %s %s", method, path)
	}
	if len(body) != 0 {
		t.Fatalf("DELETE sent a body: %q", body)
	}
	if stdout != "✓ Deleted worklog 77 on A-1\n" {
		t.Fatalf("stdout=%q", stdout)
	}
}

// S5: a --confirm value that does not repeat --id is rejected without any
// HTTP request.
func TestLogDeleteConfirmMustRepeatID(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--id", "77", "--delete", "--confirm", "78")
	if code != 1 || stdout != "" {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if stderr != "Error: --confirm must repeat the worklog id\n" {
		t.Fatalf("stderr=%q", stderr)
	}
	if requests != 0 {
		t.Fatalf("requests=%d, want 0", requests)
	}
}

// S5: `--delete` without `--id` names the missing flag and requests nothing.
func TestLogDeleteRequiresID(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--delete", "--confirm", "77")
	if code != 1 || stdout != "" {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if stderr != "Error: --delete requires --id\n" {
		t.Fatalf("stderr=%q", stderr)
	}
	if requests != 0 {
		t.Fatalf("requests=%d, want 0", requests)
	}
}

// S5: `--delete` cannot be combined with an update field.
func TestLogDeleteRejectsTimeAndComment(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "time", args: []string{"log", "A-1", "--id", "77", "--delete", "--confirm", "77", "--time", "1h"}},
		{name: "comment", args: []string{"log", "A-1", "--id", "77", "--delete", "--confirm", "77", "--comment", "note"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), tc.args...)
			if code != 1 || stdout != "" {
				t.Fatalf("code=%d stdout=%q", code, stdout)
			}
			if stderr != "Error: --delete cannot be combined with --time or --comment\n" {
				t.Fatalf("stderr=%q", stderr)
			}
			if requests != 0 {
				t.Fatalf("requests=%d, want 0", requests)
			}
		})
	}
}

// `--help` must document the new worklog management flags.
func TestLogHelpDocumentsWorklogManagementFlags(t *testing.T) {
	code, stdout, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "log", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	for _, flagName := range []string{"--id", "--delete", "--confirm", "--clear-comment"} {
		if !strings.Contains(stdout, flagName) {
			t.Fatalf("help output missing %s:\n%s", flagName, stdout)
		}
	}
	if !strings.Contains(stdout, "--show") || !strings.Contains(stdout, "cannot be combined with --id") {
		t.Fatalf("help output must document the --show/--id conflict:\n%s", stdout)
	}
}

// S13: `--show --id` is rejected before any request with the exact message.
func TestLogShowRejectsID(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--show", "--id", "77")
	if code != 1 || stdout != "" {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if stderr != "Error: --show cannot be combined with --id\n" {
		t.Fatalf("stderr=%q", stderr)
	}
	if requests != 0 {
		t.Fatalf("requests=%d, want 0", requests)
	}
}

// S14: `--clear-comment` with `--id` PUTs exactly {"comment":null} on the
// entry path and prints the standard update line.
func TestLogClearCommentWithID(t *testing.T) {
	var (
		method string
		path   string
		body   []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--id", "77", "--clear-comment")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if method != http.MethodPut || path != "/rest/api/3/issue/A-1/worklog/77" {
		t.Fatalf("request = %s %s", method, path)
	}
	if strings.TrimSpace(string(body)) != `{"comment":null}` {
		t.Fatalf("body=%q, want exactly {\"comment\":null}", body)
	}
	if stdout != "✓ Updated worklog 77 on A-1\n" {
		t.Fatalf("stdout=%q", stdout)
	}
}

// S14: `--clear-comment` without `--id` is rejected before any request.
func TestLogClearCommentRequiresID(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--clear-comment")
	if code != 1 || stdout != "" {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if stderr != "Error: --clear-comment requires --id\n" {
		t.Fatalf("stderr=%q", stderr)
	}
	if requests != 0 {
		t.Fatalf("requests=%d, want 0", requests)
	}
}

// S14: `--clear-comment` and `--comment` express contradictory intents.
func TestLogClearCommentRejectsComment(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--id", "77", "--clear-comment", "--comment", "note")
	if code != 1 || stdout != "" {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if stderr != "Error: --clear-comment cannot be combined with --comment\n" {
		t.Fatalf("stderr=%q", stderr)
	}
	if requests != 0 {
		t.Fatalf("requests=%d, want 0", requests)
	}
}

// S14: the existing delete conflict rejection must also cover
// `--clear-comment`, so `--delete` never coexists with it.
func TestLogClearCommentRejectsDelete(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--id", "77", "--delete", "--confirm", "77", "--clear-comment")
	if code != 1 || stdout != "" {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if stderr != "Error: --delete cannot be combined with --time or --comment\n" {
		t.Fatalf("stderr=%q", stderr)
	}
	if requests != 0 {
		t.Fatalf("requests=%d, want 0", requests)
	}
}

// S14 — clearing the comment while changing the duration is one mutation, so a
// failure cannot leave the worklog half-edited.
func TestLogClearCommentWithTimeSendsOneRequest(t *testing.T) {
	puts := 0
	var body map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			puts++
			if r.URL.Path != "/rest/api/3/issue/A-1/worklog/77" {
				t.Errorf("path=%s", r.URL.Path)
			}
			raw, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("body=%s err=%v", raw, err)
			}
		case http.MethodGet:
			_, _ = fmt.Fprint(w, `{"worklogs":[{"id":"77","timeSpent":"3h"}]}`)
		}
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "log", "A-1", "--id", "77", "--time", "3h", "--clear-comment")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "✓ Updated worklog 77 on A-1") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if puts != 1 {
		t.Fatalf("worklog mutations=%d, want 1", puts)
	}
	if string(body["timeSpent"]) != `"3h"` || string(body["comment"]) != "null" {
		t.Fatalf("body=%v", body)
	}
}
