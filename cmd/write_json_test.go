package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

const testIssueJSON = `{"id":"1","key":"A-1","fields":{"summary":"Result","labels":[],"components":[],"fixVersions":[]}}`

func restoreFlagDefaults(command *cobra.Command) {
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		if slice, ok := flag.Value.(pflag.SliceValue); ok {
			_ = slice.Replace(nil)
		} else {
			_ = flag.Value.Set(flag.DefValue)
		}
		flag.Changed = false
	})
	for _, child := range command.Commands() {
		restoreFlagDefaults(child)
	}
}

func runTestCLI(t *testing.T, client *jira.Client, args ...string) (int, string, string) {
	t.Helper()
	restoreFlagDefaults(rootCmd)
	var stdout, stderr bytes.Buffer
	oldBuilder := buildClient
	oldOut := rootCmd.OutOrStdout()
	oldErr := rootCmd.ErrOrStderr()
	buildClient = func() (*jira.Client, string, error) { return client, "jira.example", nil }
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	t.Cleanup(func() {
		buildClient = oldBuilder
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
		rootCmd.SetArgs(nil)
		restoreFlagDefaults(rootCmd)
	})
	code := runRootCommand(rootCmd, args)
	return code, stdout.String(), stderr.String()
}

func decodeMutationError(t *testing.T, stderr string) mutationErrorJSON {
	t.Helper()
	var envelope mutationErrorJSON
	assertSingleJSONDocument(t, []byte(stderr), &envelope)
	if envelope.SchemaVersion != "v1" {
		t.Fatalf("schemaVersion = %q, want v1", envelope.SchemaVersion)
	}
	return envelope
}

func TestWriteCommandsJSONSuccess(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "create", args: []string{"create", "--project", "A", "--summary", "Result", "--json"}},
		{name: "update", args: []string{"update", "A-1", "--summary", "Result", "--json"}},
		{name: "assign", args: []string{"assign", "A-1", "dev@example.com", "--json"}},
		{name: "move", args: []string{"move", "A-1", "--to", "Done", "--json"}},
		{name: "comment", args: []string{"comment", "A-1", "Done", "--json"}},
		{name: "log", args: []string{"log", "A-1", "--time", "30m", "--json"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutationSeen := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/rest/api/3/issue" && r.Method == http.MethodPost:
					mutationSeen = true
					_, _ = fmt.Fprint(w, `{"key":"A-1"}`)
				case r.URL.Path == "/rest/api/2/user/picker":
					_, _ = fmt.Fprint(w, `{"users":[{"accountId":"acct-1","displayName":"Developer"}]}`)
				case r.URL.Path == "/rest/api/3/user":
					_, _ = fmt.Fprint(w, `{"accountId":"acct-1","displayName":"Developer","emailAddress":"dev@example.com"}`)
				case r.URL.Path == "/rest/api/3/issue/A-1/transitions" && r.Method == http.MethodGet:
					_, _ = fmt.Fprint(w, `{"transitions":[{"id":"31","name":"Done"}]}`)
				case r.URL.Path == "/rest/api/3/issue/A-1" && r.Method == http.MethodGet:
					if !mutationSeen {
						t.Errorf("result fetched before mutation")
					}
					_, _ = fmt.Fprint(w, testIssueJSON)
				case r.Method == http.MethodPost || r.Method == http.MethodPut:
					mutationSeen = true
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), test.args...)
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if !mutationSeen {
				t.Fatal("mutation endpoint was not called")
			}
			var result map[string]any
			assertSingleJSONDocument(t, []byte(stdout), &result)
			if result["schemaVersion"] != "v1" || result["key"] != "A-1" {
				t.Fatalf("result = %#v, want TicketJSON v1 for A-1", result)
			}
		})
	}
}

func TestWriteJSONFailuresCarryMutationState(t *testing.T) {
	t.Run("pre-write validation", func(t *testing.T) {
		code, stdout, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "update", "A-1", "--json")
		if code != 1 || stdout != "" {
			t.Fatalf("exit = %d, stdout = %q", code, stdout)
		}
		envelope := decodeMutationError(t, stderr)
		if envelope.Code != "validation_error" || envelope.IssueKey != "A-1" || envelope.MutationState != jira.MutationNotApplied {
			t.Fatalf("envelope = %#v", envelope)
		}
	})

	t.Run("applied mutation refetch failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "update", "A-1", "--summary", "new", "--json")
		if code != 1 || stdout != "" {
			t.Fatalf("exit = %d, stdout = %q", code, stdout)
		}
		envelope := decodeMutationError(t, stderr)
		if envelope.Code != "refetch_failed" || envelope.MutationState != jira.MutationApplied {
			t.Fatalf("envelope = %#v", envelope)
		}
	})

	t.Run("transport ambiguity", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hijacker := w.(http.Hijacker)
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.Close()
		}))
		defer srv.Close()
		code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "update", "A-1", "--summary", "new", "--json")
		if code != 1 || stdout != "" {
			t.Fatalf("exit = %d, stdout = %q", code, stdout)
		}
		envelope := decodeMutationError(t, stderr)
		if envelope.Code != "mutation_failed" || envelope.MutationState != jira.MutationUnknown {
			t.Fatalf("envelope = %#v", envelope)
		}
	})

	t.Run("create sprint partial failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/rest/api/3/issue" {
				_, _ = fmt.Fprint(w, `{"key":"A-9"}`)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "create", "--project", "A", "--summary", "partial", "--board", "3", "--json")
		if code != 1 || stdout != "" {
			t.Fatalf("exit = %d, stdout = %q", code, stdout)
		}
		envelope := decodeMutationError(t, stderr)
		if envelope.Code != "partial_failure" || envelope.IssueKey != "A-9" || envelope.MutationState != jira.MutationApplied {
			t.Fatalf("envelope = %#v", envelope)
		}
	})
}

func TestWriteCommandsPreserveHumanDefaults(t *testing.T) {
	getCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			getCalls++
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "update", "A-1", "--summary", "new")
	if code != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if stdout != "✓ Updated A-1: summary\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if getCalls != 0 {
		t.Fatalf("human mode made %d result refetches, want 0", getCalls)
	}
}

func TestExistingReadJSONShapesRemainArrays(t *testing.T) {
	tests := []struct {
		name string
		args []string
		body string
	}{
		{name: "move transitions", args: []string{"move", "A-1", "--json"}, body: `{"transitions":[{"id":"1","name":"Done"}]}`},
		{name: "worklog show", args: []string{"log", "A-1", "--show", "--json"}, body: `{"worklogs":[{"timeSpent":"30m"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, test.body) }))
			defer srv.Close()
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), test.args...)
			if code != 0 || stderr != "" {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			var result []any
			assertSingleJSONDocument(t, []byte(stdout), &result)
			if len(result) != 1 {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestMutationResultOutputFailureIsPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, testIssueJSON)
	}))
	defer srv.Close()
	var stderr bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(failingWriter{err: fmt.Errorf("stdout unavailable")})
	command.SetErr(&stderr)

	err := writeMutationResult(command, jira.NewClient(srv.URL, "email", "token"), "jira.example", "A-1")
	if !isReportedCommandError(err) {
		t.Fatalf("error = %T %v, want reported output error", err, err)
	}
	envelope := decodeMutationError(t, stderr.String())
	if envelope.Code != "output_failed" || envelope.MutationState != jira.MutationApplied {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestMalformedFlagsUseCommandAwareJSONIntentWithoutMutation(t *testing.T) {
	const sensitiveValue = "sensitive-invalid-value"
	tests := []struct {
		name     string
		args     []string
		wantJSON bool
	}{
		{name: "implicit true after malformed value", args: []string{"create", "--board", sensitiveValue, "--json"}, wantJSON: true},
		{name: "numeric true after malformed value", args: []string{"create", "--board", sensitiveValue, "--json=1"}, wantJSON: true},
		{name: "uppercase true after malformed value", args: []string{"create", "--board", sensitiveValue, "--json=TRUE"}, wantJSON: true},
		{name: "true before command", args: []string{"--json=true", "create", "--board", sensitiveValue}, wantJSON: true},
		{name: "true before malformed value", args: []string{"create", "--json=T", "--board", sensitiveValue}, wantJSON: true},
		{name: "before-command false overridden after command", args: []string{"--json=false", "create", "--board", sensitiveValue, "--json=TRUE"}, wantJSON: true},
		{name: "later true overrides false beyond malformed value", args: []string{"create", "--json=false", "--board", sensitiveValue, "--json=true"}, wantJSON: true},
		{name: "JSON token is a summary value", args: []string{"create", "--summary", "--json"}, wantJSON: false},
		{name: "JSON token follows terminator", args: []string{"create", "--summary", "result", "--", "--json"}, wantJSON: false},
		{name: "later false beyond malformed value overrides true", args: []string{"create", "--json", "--board", sensitiveValue, "--json=false"}, wantJSON: false},
		{name: "after-command false overrides before-command true", args: []string{"--json=1", "create", "--board", sensitiveValue, "--json=false"}, wantJSON: false},
		{name: "later numeric false before malformed value", args: []string{"create", "--json=TRUE", "--json=0", "--board", sensitiveValue}, wantJSON: false},
		{name: "explicit false after malformed value", args: []string{"create", "--board", sensitiveValue, "--json=FALSE"}, wantJSON: false},
		{name: "invalid JSON boolean is human and sanitized", args: []string{"create", "--json=json-secret-value"}, wantJSON: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()

			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), test.args...)
			if code != 1 || stdout != "" {
				t.Fatalf("exit = %d, stdout = %q", code, stdout)
			}
			if requests != 0 {
				t.Fatalf("malformed command made %d remote requests, want 0", requests)
			}
			if strings.Contains(stderr, sensitiveValue) || strings.Contains(stderr, "json-secret-value") || strings.Contains(stderr, "Usage:") {
				t.Fatalf("stderr leaked raw Cobra diagnostics: %q", stderr)
			}
			if test.wantJSON {
				if strings.Contains(stderr, "Error:") {
					t.Fatalf("JSON stderr contains human diagnostics: %q", stderr)
				}
				envelope := decodeMutationError(t, stderr)
				if envelope.Code != "validation_error" || envelope.MutationState != jira.MutationNotApplied {
					t.Fatalf("envelope = %#v", envelope)
				}
			} else {
				if !strings.HasPrefix(stderr, "Error: ") || strings.Contains(stderr, `"schemaVersion"`) {
					t.Fatalf("human stderr = %q, want one sanitized human error", stderr)
				}
			}
		})
	}
}

func TestRootJSONErrorIsExclusiveAndReturnsExitOne(t *testing.T) {
	code, stdout, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "update", "--json")
	if code != 1 || stdout != "" {
		t.Fatalf("exit = %d, stdout = %q", code, stdout)
	}
	if strings.Contains(stderr, "Error:") || strings.Contains(stderr, "Usage:") {
		t.Fatalf("stderr contains duplicate Cobra text: %q", stderr)
	}
	envelope := decodeMutationError(t, stderr)
	if envelope.Code != "validation_error" || envelope.MutationState != jira.MutationNotApplied {
		t.Fatalf("envelope = %#v", envelope)
	}
}
