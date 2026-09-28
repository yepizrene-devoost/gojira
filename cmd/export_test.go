package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestWriteExportJSONUsesCobraOutputAsTopLevelArray(t *testing.T) {
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	issues := []jira.Issue{{Key: "ARA-10"}}

	if err := writeExportJSON(cmd, issues, "example.atlassian.net"); err != nil {
		t.Fatalf("writeExportJSON() error = %v", err)
	}

	var tickets []jira.TicketJSON
	assertSingleJSONDocument(t, stdout.Bytes(), &tickets)
	if len(tickets) != 1 || tickets[0].Key != "ARA-10" || tickets[0].SchemaVersion != "v1" {
		t.Fatalf("tickets = %#v, want one v1 ARA-10 ticket", tickets)
	}
}

// Export's automatic choices write diagnostics to process stderr, while its
// JSON error envelope uses Cobra's error writer. Capture both streams together.
func runExportCLI(t *testing.T, args []string, builder func() (*jira.Client, string, error), writer io.Writer) (int, string) {
	t.Helper()
	reader, pipeWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = pipeWriter
	defer func() {
		os.Stderr = oldStderr
		_ = pipeWriter.Close()
		_ = reader.Close()
	}()
	code, cobraStderr := runReadCLI(t, args, builder, writer)
	os.Stderr = oldStderr
	if err := pipeWriter.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}
	diagnostics, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr pipe: %v", err)
	}
	return code, string(diagnostics) + cobraStderr
}

func TestExportJSONChoices(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		responses  []string
		wantStderr string
		wantCalls  int
	}{
		{"explicit board and sprint", []string{"export", "--board", "7", "--sprint", "9"}, []string{`{"issues":[{"key":"ARA-10","fields":{"summary":"One"}}],"total":1}`}, "", 1},
		{"automatic board and active sprint", []string{"export"}, []string{`{"values":[{"id":7,"name":"Team"}]}`, `{"values":[{"id":9,"name":"Current","state":"active"}]}`, `{"issues":[{"key":"ARA-10"}],"total":1}`}, "Using board: Team (id=7)\nUsing sprint: Current (id=9)\n", 3},
		{"explicit board and no active sprint", []string{"export", "--board", "7"}, []string{`{"values":[]}`, `{"values":[{"id":8,"name":"Future","state":"future"}]}`, `{"issues":[],"total":0}`}, "", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				if calls >= len(tc.responses) {
					t.Errorf("unexpected request %s", r.URL)
					return
				}
				_, _ = fmt.Fprint(w, tc.responses[calls])
				calls++
			}))
			defer srv.Close()
			var stdout bytes.Buffer
			code, stderr := runExportCLI(t, tc.args, func() (*jira.Client, string, error) {
				return jira.NewClient(srv.URL, "email", "token"), "jira.example", nil
			}, &stdout)
			if code != 0 || stderr != tc.wantStderr || calls != tc.wantCalls {
				t.Fatalf("exit=%d stderr=%q calls=%d; want 0, %q, %d", code, stderr, calls, tc.wantStderr, tc.wantCalls)
			}
			var tickets []jira.TicketJSON
			assertSingleJSONDocument(t, stdout.Bytes(), &tickets)
			wantTickets := 1
			if tc.name == "explicit board and no active sprint" {
				wantTickets = 0
			}
			if len(tickets) != wantTickets {
				t.Fatalf("tickets = %+v, want %d", tickets, wantTickets)
			}
			if wantTickets == 1 && (tickets[0].Key != "ARA-10" || tickets[0].SchemaVersion != "v1") {
				t.Fatalf("ticket = %+v, want v1 ARA-10", tickets[0])
			}
		})
	}
}

func TestExportJSONFailures(t *testing.T) {
	for _, tc := range []struct {
		name, response, wantCode  string
		args                      []string
		status                    int
		builderError, writerError error
	}{
		{name: "configuration", args: []string{"export", "--board", "7", "--sprint", "9"}, wantCode: "configuration_error", builderError: errors.New("client unavailable")},
		{name: "boards HTTP 4xx", args: []string{"export"}, wantCode: "read_failed", status: http.StatusForbidden},
		{name: "boards parse", args: []string{"export"}, wantCode: "read_failed", response: `{broken`},
		{name: "no boards", args: []string{"export"}, wantCode: "validation_error", response: `{"values":[]}`},
		{name: "sprints HTTP 5xx", args: []string{"export", "--board", "7"}, wantCode: "read_failed", status: http.StatusInternalServerError},
		{name: "sprints parse", args: []string{"export", "--board", "7"}, wantCode: "read_failed", response: `{broken`},
		{name: "issues HTTP 4xx", args: []string{"export", "--board", "7", "--sprint", "9"}, wantCode: "read_failed", status: http.StatusForbidden},
		{name: "issues HTTP 5xx", args: []string{"export", "--board", "7", "--sprint", "9"}, wantCode: "read_failed", status: http.StatusInternalServerError},
		{name: "issues parse", args: []string{"export", "--board", "7", "--sprint", "9"}, wantCode: "read_failed", response: `{broken`},
		{name: "writer", args: []string{"export", "--board", "7", "--sprint", "9"}, wantCode: "output_failed", response: `{"issues":[{"key":"ARA-10"}],"total":1}`, writerError: errors.New("output unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				_, _ = fmt.Fprint(w, tc.response)
			}))
			defer srv.Close()
			var stdout bytes.Buffer
			var writer io.Writer = &stdout
			if tc.writerError != nil {
				writer = failingWriter{err: tc.writerError}
			}
			code, stderr := runExportCLI(t, tc.args, func() (*jira.Client, string, error) {
				if tc.builderError != nil {
					return nil, "", tc.builderError
				}
				return jira.NewClient(srv.URL, "email", "token"), "jira.example", nil
			}, writer)
			if code != 1 || stdout.Len() != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr)
			}
			if tc.builderError != nil && requests != 0 || tc.builderError == nil && requests < 1 {
				t.Fatalf("requests=%d", requests)
			}
			if strings.Contains(stderr, "Error:") {
				t.Fatalf("plain root error in stderr: %q", stderr)
			}
			got := decodeMutationError(t, stderr)
			if got.Code != tc.wantCode || got.MutationState != jira.MutationNotApplied || got.IssueKey != "" || got.SchemaVersion != "v1" {
				t.Fatalf("error=%+v, want %s/not_applied/no issueKey/v1", got, tc.wantCode)
			}
		})
	}
}
