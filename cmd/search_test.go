package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
		{name: "HTTP 4xx", wantCode: "read_failed", status: http.StatusForbidden},
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
				if got.Code != tc.wantCode || got.MutationState != jira.MutationNotApplied || got.IssueKey != "" {
					t.Fatalf("error=%+v, want %s/not_applied/no key", got, tc.wantCode)
				}
			})
		}
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
