package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// runTestCLIWithOut behaves like runTestCLI but lets the caller replace stdout.
func runTestCLIWithOut(t *testing.T, client *jira.Client, out failingWriter, args ...string) (int, string) {
	t.Helper()
	restoreFlagDefaults(rootCmd)
	var stderr strings.Builder
	oldBuilder := buildClient
	oldOut := rootCmd.OutOrStdout()
	oldErr := rootCmd.ErrOrStderr()
	buildClient = func() (*jira.Client, string, error) { return client, "jira.example", nil }
	rootCmd.SetOut(out)
	rootCmd.SetErr(&stderr)
	t.Cleanup(func() {
		buildClient = oldBuilder
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
		rootCmd.SetArgs(nil)
		restoreFlagDefaults(rootCmd)
	})
	return runRootCommand(rootCmd, args), stderr.String()
}

func TestJSONReadFailuresUseV1EnvelopeForEveryJSONCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tests := []struct {
		name string
		args []string
		code string
	}{
		{"move lists transitions", []string{"move", "A-1", "--json"}, "read_failed"},
		{"log shows worklogs", []string{"log", "A-1", "--show", "--json"}, "read_failed"},
		{"get", []string{"get", "A-1", "--json"}, "read_failed"},
		{"search", []string{"search", "project = A", "--json"}, "read_failed"},
		{"boards", []string{"boards", "--json"}, "read_failed"},
		{"projects", []string{"projects", "--json"}, "read_failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), tt.args...)
			if code != 1 || stdout != "" {
				t.Fatalf("exit = %d, stdout = %q", code, stdout)
			}
			envelope := decodeMutationError(t, stderr)
			if envelope.Code != tt.code || envelope.MutationState != jira.MutationNotApplied {
				t.Fatalf("envelope = %#v", envelope)
			}
		})
	}
}

func TestJSONOutputWriterFailureReportsOutputFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/transitions"):
			_, _ = fmt.Fprint(w, `{"transitions":[{"id":"1","name":"Done"}]}`)
		case strings.HasSuffix(r.URL.Path, "/worklog"):
			_, _ = fmt.Fprint(w, `{"worklogs":[]}`)
		default:
			_, _ = fmt.Fprint(w, testIssueJSON)
		}
	}))
	defer srv.Close()

	tests := []struct {
		name string
		args []string
	}{
		{"version", []string{"version", "--json"}},
		{"move lists transitions", []string{"move", "A-1", "--json"}},
		{"log shows worklogs", []string{"log", "A-1", "--show", "--json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stderr := runTestCLIWithOut(t, jira.NewClient(srv.URL, "email", "token"), failingWriter{err: fmt.Errorf("stdout unavailable")}, tt.args...)
			if code != 1 {
				t.Fatalf("exit = %d, want 1", code)
			}
			envelope := decodeMutationError(t, stderr)
			if envelope.Code != "output_failed" || envelope.MutationState != jira.MutationNotApplied {
				t.Fatalf("envelope = %#v", envelope)
			}
		})
	}
}

func TestEmptyListResponsesStayJSONArrays(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	tests := []struct {
		name string
		args []string
	}{
		{"transitions", []string{"move", "A-1", "--json"}},
		{"worklogs", []string{"log", "A-1", "--show", "--json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), tt.args...)
			if code != 0 || stderr != "" {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			if strings.TrimSpace(stdout) != "[]" {
				t.Fatalf("stdout = %q, want []", stdout)
			}
		})
	}
}
