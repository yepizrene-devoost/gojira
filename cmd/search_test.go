package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestSearchJSONErrors(t *testing.T) {
	for _, tc := range []struct {
		name, response, wantCode  string
		status                    int
		builderError, writerError error
	}{
		{name: "configuration", wantCode: "configuration_error", builderError: errors.New("client unavailable")},
		{name: "HTTP 4xx", wantCode: "permission_denied", status: http.StatusForbidden},
		{name: "HTTP 5xx", wantCode: "read_failed", status: http.StatusInternalServerError},
		{name: "invalid response", wantCode: "read_failed", response: `{broken`},
		{name: "writer", wantCode: "output_failed", response: `{"issues":[],"total":0}`, writerError: errors.New("output unavailable")},
	} {
		for _, before := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json-before=%t", tc.name, before), func(t *testing.T) {
				requests := 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					if r.Method != http.MethodGet {
						t.Errorf("method = %s", r.Method)
					}
					if tc.status != 0 {
						w.WriteHeader(tc.status)
					}
					_, _ = fmt.Fprint(w, tc.response)
				}))
				defer srv.Close()
				args := []string{"search", "project = A", "--json"}
				if before {
					args = []string{"search", "--json", "project = A"}
				}
				var stdout bytes.Buffer
				var writer io.Writer = &stdout
				if tc.writerError != nil {
					writer = failingWriter{err: tc.writerError}
				}
				code, stderr := runReadCLI(t, args, func() (*jira.Client, string, error) {
					if tc.builderError != nil {
						return nil, "", tc.builderError
					}
					return jira.NewClient(srv.URL, "email", "token"), "jira.example", nil
				}, writer)
				if code != 1 || stdout.Len() != 0 {
					t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr)
				}
				wantRequests := 1
				if tc.builderError != nil {
					wantRequests = 0
				}
				if (tc.status == http.StatusInternalServerError && requests < 1) || (tc.status != http.StatusInternalServerError && requests != wantRequests) {
					t.Fatalf("requests=%d, want %d (5xx may retry)", requests, wantRequests)
				}
				got := decodeMutationError(t, stderr)
				if tc.status == http.StatusForbidden && (got.HTTPStatus != tc.status || got.Hint == "") {
					t.Fatalf("missing access details: %+v", got)
				}
				if got.Code != tc.wantCode || got.MutationState != jira.MutationNotApplied || got.IssueKey != "" {
					t.Fatalf("error=%+v, want %s/not_applied/no key", got, tc.wantCode)
				}
			})
		}
	}
}

func TestSearchCompletenessCLI(t *testing.T) {
	for _, tc := range []struct {
		name          string
		flags         []string
		response      string
		warning, meta bool
		code          int
	}{
		{"complete", []string{"--json"}, `{"issues":[],"isLast":true}`, false, false, 0},
		{"truncated", []string{"--json", "--limit", "1"}, `{"issues":[{"key":"A-1"}],"nextPageToken":"next"}`, true, false, 0},
		{"metadata empty", []string{"--meta"}, `{"issues":[],"isLast":true}`, false, true, 0},
		{"conflict", []string{"--json", "--all", "--limit", "50"}, ``, false, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method == http.MethodPost {
					_, _ = fmt.Fprint(w, `{"count":10}`)
					return
				}
				_, _ = fmt.Fprint(w, tc.response)
			}))
			defer srv.Close()
			var stdout bytes.Buffer
			code, stderr := runReadCLI(t, append([]string{"search", "project = A"}, tc.flags...), func() (*jira.Client, string, error) {
				return jira.NewClient(srv.URL, "email", "token"), "jira.example", nil
			}, &stdout)
			if code != tc.code {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
			if tc.code == 1 {
				if requests != 0 || decodeMutationError(t, stderr).Code != "validation_error" {
					t.Fatalf("requests=%d stderr=%s", requests, stderr)
				}
				return
			}
			if tc.meta {
				var envelope struct {
					SchemaVersion string
					Issues        []jira.TicketJSON
					Returned      int
					Truncated     bool
				}
				assertSingleJSONDocument(t, stdout.Bytes(), &envelope)
				if envelope.SchemaVersion != "v1" || envelope.Issues == nil || envelope.Returned != 0 || envelope.Truncated {
					t.Fatalf("envelope=%+v", envelope)
				}
			} else {
				var tickets []jira.TicketJSON
				assertSingleJSONDocument(t, stdout.Bytes(), &tickets)
			}
			if tc.warning {
				var warning struct {
					Code           string
					Returned       int
					EstimatedTotal int
					Hint           string
				}
				assertSingleJSONDocument(t, []byte(stderr), &warning)
				if warning.Code != "results_truncated" || warning.Returned != 1 || warning.EstimatedTotal != 10 || warning.Hint == "" {
					t.Fatalf("warning=%+v", warning)
				}
			} else if stderr != "" {
				t.Fatalf("stderr=%s", stderr)
			}
		})
	}
}

func TestSearchAllAndHuman(t *testing.T) {
	for _, tc := range []struct {
		name             string
		total            int
		flags            []string
		want             int
		truncated, human bool
	}{
		{"all pages", 250, []string{"--all", "--meta"}, 250, false, false},
		{"all cap", 10001, []string{"--all", "--meta"}, 10000, true, false},
		{"human complete", 2, nil, 2, false, true},
		{"human truncated", 60, nil, 50, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			returned := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				var size int
				_, _ = fmt.Sscan(r.URL.Query().Get("maxResults"), &size)
				n := min(size, tc.total-returned)
				returned += n
				_ = json.NewEncoder(w).Encode(map[string]any{"issues": make([]jira.Issue, n), "isLast": returned == tc.total, "nextPageToken": fmt.Sprint(returned)})
			}))
			defer srv.Close()
			var stdout bytes.Buffer
			code, stderr := runReadCLI(t, append([]string{"search", "project = A"}, tc.flags...), func() (*jira.Client, string, error) {
				return jira.NewClient(srv.URL, "email", "token"), "jira.example", nil
			}, &stdout)
			if code != 0 {
				t.Fatalf("exit=%d stderr=%s", code, stderr)
			}
			if tc.human {
				if !strings.Contains(stdout.String(), fmt.Sprintf("Showing %d issues", tc.want)) || strings.Contains(stdout.String(), "Found") || strings.Contains(stdout.String(), "... and") || strings.Contains(stdout.String(), "in total") {
					t.Fatalf("output=%s", stdout.String())
				}
				if strings.Contains(stderr, "Warning:") != tc.truncated {
					t.Fatalf("stderr=%s", stderr)
				}
			} else {
				var envelope struct {
					Issues         []jira.TicketJSON
					Returned       int
					Truncated      bool
					EstimatedTotal *int
				}
				assertSingleJSONDocument(t, stdout.Bytes(), &envelope)
				if len(envelope.Issues) != tc.want || envelope.Returned != tc.want || envelope.Truncated != tc.truncated || envelope.EstimatedTotal != nil || stderr != "" {
					t.Fatalf("returned=%d truncated=%t stderr=%s", envelope.Returned, envelope.Truncated, stderr)
				}
			}
		})
	}
}

func TestWriteSearchJSONUsesCobraOutputAsTopLevelArray(t *testing.T) {
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	issues := []jira.Issue{{Key: "ARA-1"}, {Key: "ARA-2"}}

	if err := writeSearchJSON(cmd, issues, "example.atlassian.net"); err != nil {
		t.Fatalf("writeSearchJSON() error = %v", err)
	}

	var tickets []jira.TicketJSON
	assertSingleJSONDocument(t, stdout.Bytes(), &tickets)
	if len(tickets) != 2 {
		t.Fatalf("len(tickets) = %d, want 2", len(tickets))
	}
	for i, ticket := range tickets {
		if ticket.SchemaVersion != "v1" {
			t.Errorf("ticket %d SchemaVersion = %q, want v1", i, ticket.SchemaVersion)
		}
	}
}

func TestSearchAllWithLimitFailsInHumanMode(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = fmt.Fprint(w, `{"issues":[],"isLast":true}`)
	}))
	defer srv.Close()

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "search", "x", "--all", "--limit", "5")
	if code != 1 || stdout != "" || requests != 0 {
		t.Fatalf("exit = %d, stdout = %q, requests = %d, want exit 1 with no output and no requests", code, stdout, requests)
	}
	if !strings.Contains(stderr, "--all cannot be combined with --limit") {
		t.Fatalf("stderr = %q, want the conflict message", stderr)
	}
}
