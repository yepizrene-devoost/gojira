package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestProjectsJSONList(t *testing.T) {
	for _, tc := range []struct {
		name, response, want string
	}{
		{"nonempty raw projects", `[{"key":"A","name":"Alpha"}]`, `[{"key":"A","name":"Alpha"}]`},
		{"empty projects", `[]`, `[]`},
		{"null projects", `null`, `[]`},
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
			code, stderr := runListCLI(t, "projects", func() (*jira.Client, string, error) {
				return jira.NewClient(srv.URL, "email", "token"), "jira.example", nil
			}, &stdout, true)
			if code != 0 || stderr != "" || requests != 1 {
				t.Fatalf("exit = %d, stderr = %q, requests = %d", code, stderr, requests)
			}
			var projects []jira.Project
			assertSingleJSONDocument(t, stdout.Bytes(), &projects)
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

func TestProjectsHumanOutputUnchanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[{"key":"A","name":"Alpha"},{"key":"LONGKEY","name":"Second"}]`)
	}))
	defer srv.Close()
	var cobraOutput bytes.Buffer
	code, stdout, stderr := captureListStdout(t, func() (int, string) {
		return runListCLI(t, "projects", func() (*jira.Client, string, error) {
			return jira.NewClient(srv.URL, "email", "token"), "jira.example", nil
		}, &cobraOutput, false)
	})
	want := "A         Alpha\nLONGKEY   Second\n"
	if code != 0 || stderr != "" || stdout != want || cobraOutput.Len() != 0 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q, Cobra output = %q; want stdout %q", code, stdout, stderr, cobraOutput.String(), want)
	}
}

func TestProjectsJSONFailure(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		builderError   error
		writerError    error
		status         int
	}{
		{name: "client failure", builderError: errors.New("client unavailable")},
		{name: "parse failure", response: `{broken`},
		{name: "HTTP 4xx", status: http.StatusForbidden, response: `denied`},
		{name: "HTTP 5xx", status: http.StatusInternalServerError, response: `unavailable`},
		{name: "writer failure", response: `[{"key":"A","name":"Alpha"}]`, writerError: errors.New("output unavailable")},
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
			code, stderr := runListCLI(t, "projects", func() (*jira.Client, string, error) {
				if tc.builderError != nil {
					return nil, "", tc.builderError
				}
				return jira.NewClient(srv.URL, "email", "token"), "jira.example", nil
			}, writer, true)
			if code != 1 || stdout.Len() != 0 {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr)
			}
			if got := decodeMutationError(t, stderr); got.Code != "validation_error" {
				t.Fatalf("error code = %q, want validation_error", got.Code)
			}
		})
	}
}
