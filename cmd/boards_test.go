package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// runListCLI isolates the root's output and client seam for list command tests.
func runListCLI(t *testing.T, command string, builder func() (*jira.Client, string, error), writer io.Writer, jsonOutput bool) (int, string) {
	t.Helper()
	restoreFlagDefaults(rootCmd)
	var stderr bytes.Buffer
	oldBuilder := buildClient
	oldOut := rootCmd.OutOrStdout()
	oldErr := rootCmd.ErrOrStderr()
	buildClient = builder
	rootCmd.SetOut(writer)
	rootCmd.SetErr(&stderr)
	defer func() {
		buildClient = oldBuilder
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
		rootCmd.SetArgs(nil)
		restoreFlagDefaults(rootCmd)
	}()
	args := []string{command}
	if jsonOutput {
		args = append(args, "--json")
	}
	return runRootCommand(rootCmd, args), stderr.String()
}

// captureListStdout checks the existing human-mode fmt.Printf output while
// restoring process stdout before assertions; these tests do not run in parallel.
func captureListStdout(t *testing.T, run func() (int, string)) (int, string, string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	defer reader.Close()
	original := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = original
		writer.Close()
	}()
	code, stderr := run()
	os.Stdout = original
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return code, string(data), stderr
}

func TestBoardsJSONList(t *testing.T) {
	for _, tc := range []struct {
		name, response, want string
	}{
		{"nonempty raw boards", `{"values":[{"id":7,"name":"Team","type":"scrum","location":{"projectName":"Alpha","projectKey":"A"}}]}`, `[{"id":7,"name":"Team","type":"scrum","location":{"projectName":"Alpha","projectKey":"A"}}]`},
		{"empty boards", `{"values":[]}`, `[]`},
		{"missing values", `{}`, `[]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				_, _ = fmt.Fprint(w, tc.response)
			}))
			defer srv.Close()
			var stdout bytes.Buffer
			code, stderr := runListCLI(t, "boards", func() (*jira.Client, string, error) {
				return jira.NewClient(srv.URL, "email", "token"), "jira.example", nil
			}, &stdout, true)
			if code != 0 || stderr != "" || requests != 1 {
				t.Fatalf("exit = %d, stderr = %q, requests = %d", code, stderr, requests)
			}
			var boards []jira.Board
			assertSingleJSONDocument(t, stdout.Bytes(), &boards)
			var compact bytes.Buffer
			if err := json.Compact(&compact, stdout.Bytes()); err != nil {
				t.Fatalf("compact stdout: %v", err)
			}
			if compact.String() != tc.want {
				t.Fatalf("stdout = %q, want %q", compact.String(), tc.want)
			}
		})
	}
}

func TestBoardsHumanOutputUnchanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"values":[{"id":7,"name":"Team","type":"scrum","location":{"projectName":"Alpha","projectKey":"A"}},{"id":8,"name":"Other","type":"kanban"}]}`)
	}))
	defer srv.Close()
	var cobraOutput bytes.Buffer
	code, stdout, stderr := captureListStdout(t, func() (int, string) {
		return runListCLI(t, "boards", func() (*jira.Client, string, error) {
			return jira.NewClient(srv.URL, "email", "token"), "jira.example", nil
		}, &cobraOutput, false)
	})
	want := fmt.Sprintf("%-6d  %-30s  [%s]  (%s)\n%-6d  %-30s  [%s]\n", 7, "Team", "scrum", "Alpha", 8, "Other", "kanban")
	if code != 0 || stderr != "" || stdout != want || cobraOutput.Len() != 0 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q, Cobra output = %q; want stdout %q", code, stdout, stderr, cobraOutput.String(), want)
	}
}

func TestBoardsJSONFailure(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		wantCode       string
		builderError   error
		writerError    error
		status         int
	}{
		{name: "client failure", wantCode: "configuration_error", builderError: errors.New("client unavailable")},
		{name: "parse failure", wantCode: "read_failed", response: `{broken`},
		{name: "HTTP 4xx", wantCode: "read_failed", status: http.StatusForbidden, response: `denied`},
		{name: "HTTP 5xx", wantCode: "read_failed", status: http.StatusInternalServerError, response: `unavailable`},
		{name: "writer failure", wantCode: "output_failed", response: `{"values":[{"id":7,"name":"Team","type":"scrum"}]}`, writerError: errors.New("output unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			code, stderr := runListCLI(t, "boards", func() (*jira.Client, string, error) {
				if tc.builderError != nil {
					return nil, "", tc.builderError
				}
				return jira.NewClient(srv.URL, "email", "token"), "jira.example", nil
			}, writer, true)
			if code != 1 || stdout.Len() != 0 {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr)
			}
			if got := decodeMutationError(t, stderr); got.Code != tc.wantCode || got.MutationState != jira.MutationNotApplied || got.IssueKey != "" {
				t.Fatalf("error = %+v, want %s/not_applied without issueKey", got, tc.wantCode)
			}
		})
	}
}
