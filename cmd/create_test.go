package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestCreateParentPayloadAndDefaults(t *testing.T) {
	tests := []struct {
		name, parent, issueType string
		args                    []string
		wantJSON                bool
	}{
		{name: "ordinary defaults to Task", issueType: "Task"},
		{name: "parent defaults to Sub-task", parent: "ARA-12", issueType: "Sub-task", args: []string{"--parent", "ARA-12"}},
		{name: "explicit compatible type", parent: "ARA-12", issueType: "Sub-task", args: []string{"--parent", "ARA-12", "--type", "Sub-task"}, wantJSON: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var writes, reads int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/issue":
					writes++
					var body struct {
						Fields map[string]json.RawMessage `json:"fields"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("decode: %v", err)
					}
					var issueType struct {
						Name string `json:"name"`
					}
					if err := json.Unmarshal(body.Fields["issuetype"], &issueType); err != nil {
						t.Errorf("type: %v", err)
					}
					if issueType.Name != tc.issueType {
						t.Errorf("type = %q, want %q", issueType.Name, tc.issueType)
					}
					if tc.parent == "" {
						if _, ok := body.Fields["parent"]; ok {
							t.Error("ordinary create included parent")
						}
					} else {
						var parent map[string]string
						if err := json.Unmarshal(body.Fields["parent"], &parent); err != nil {
							t.Errorf("parent: %v", err)
						}
						if len(parent) != 1 || parent["key"] != tc.parent {
							t.Errorf("parent fields = %#v", parent)
						}
					}
					var desc jira.ADFDoc
					if err := json.Unmarshal(body.Fields["description"], &desc); err != nil || desc.Flatten() != "Follow up" {
						t.Errorf("description = %q, err = %v", desc.Flatten(), err)
					}
					_, _ = fmt.Fprint(w, `{"key":"ARA-13"}`)
				case r.Method == http.MethodGet && tc.wantJSON:
					reads++
					_, _ = fmt.Fprint(w, `{"key":"ARA-13","fields":{"summary":"Follow up"}}`)
				default:
					t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()
			file := filepath.Join(t.TempDir(), "description.md")
			if err := os.WriteFile(file, []byte("Follow up"), 0600); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"create", "--project", "ARA", "--summary", "Follow up", "--description-file", file}, tc.args...)
			if tc.wantJSON {
				args = append(args, "--json")
			}
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), args...)
			if code != 0 || stderr != "" || writes != 1 {
				t.Fatalf("code=%d stderr=%q writes=%d", code, stderr, writes)
			}
			if tc.wantJSON {
				var result map[string]any
				assertSingleJSONDocument(t, []byte(stdout), &result)
				if result["schemaVersion"] != "v1" || result["key"] != "ARA-13" || reads != 1 {
					t.Fatalf("result=%#v reads=%d", result, reads)
				}
			} else if stdout != "✓ Created ARA-13\n" || reads != 0 {
				t.Fatalf("stdout=%q reads=%d", stdout, reads)
			}
		})
	}
}

func TestCreateParentValidationBeforeClient(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"empty", []string{"--parent="}},
		{"whitespace", []string{"--parent", " ARA-1"}},
		{"malformed", []string{"--parent", "ARA-0"}},
		{"injection", []string{"--parent", "ARA-1/extra"}},
		{"wrong type", []string{"--parent", "ARA-1", "--type", "Task"}},
		{"empty explicit type", []string{"--parent", "ARA-1", "--type="}},
		{"board", []string{"--parent", "ARA-1", "--board", "1"}},
		{"explicit zero board", []string{"--parent", "ARA-1", "--board", "0"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()
			args := append([]string{"create", "--project", "ARA", "--summary", "child", "--json"}, tc.args...)
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), args...)
			if code != 1 || stdout != "" || requests != 0 {
				t.Fatalf("code=%d stdout=%q requests=%d", code, stdout, requests)
			}
			envelope := decodeMutationError(t, stderr)
			if envelope.Code != "validation_error" || envelope.MutationState != jira.MutationNotApplied {
				t.Fatalf("envelope=%#v", envelope)
			}
		})
	}
}

func TestCreateSubtaskValidationSkipsClientSetup(t *testing.T) {
	restoreFlagDefaults(rootCmd)
	oldBuilder := buildClient
	oldOut, oldErr := rootCmd.OutOrStdout(), rootCmd.ErrOrStderr()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	builds := 0
	buildClient = func() (*jira.Client, string, error) {
		builds++
		return nil, "", fmt.Errorf("client setup must not run")
	}
	t.Cleanup(func() {
		buildClient = oldBuilder
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
		rootCmd.SetArgs(nil)
		restoreFlagDefaults(rootCmd)
	})
	if code := runRootCommand(rootCmd, []string{"create", "--project", "ARA", "--summary", "child", "--parent", "ARA-1", "--type", "Task", "--json"}); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if builds != 0 || stdout.Len() != 0 {
		t.Fatalf("builds = %d, stdout = %q", builds, stdout.String())
	}
	if envelope := decodeMutationError(t, stderr.String()); envelope.Code != "validation_error" {
		t.Fatalf("error = %#v", envelope)
	}
}

func TestCreateSubtaskFailureStates(t *testing.T) {
	for _, tc := range []struct {
		name, body, wantCode, wantKey string
		status                        int
		wantState                     jira.MutationState
	}{
		{"rejected", `{}`, "mutation_failed", "", 400, jira.MutationNotApplied},
		{"server failure", `{}`, "mutation_failed", "", 503, jira.MutationUnknown},
		{"bad response", `{}`, "mutation_failed", "", 201, jira.MutationApplied},
		{"refetch failed", `{"key":"ARA-13"}`, "refetch_failed", "ARA-13", 201, jira.MutationApplied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
					w.WriteHeader(tc.status)
					_, _ = fmt.Fprint(w, tc.body)
					return
				}
				w.WriteHeader(http.StatusBadRequest)
			}))
			defer srv.Close()
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "create", "--project", "ARA", "--summary", "child", "--parent", "ARA-1", "--json")
			if code != 1 || stdout != "" || posts != 1 {
				t.Fatalf("code=%d stdout=%q posts=%d", code, stdout, posts)
			}
			envelope := decodeMutationError(t, stderr)
			if envelope.Code != tc.wantCode || envelope.MutationState != tc.wantState || envelope.IssueKey != tc.wantKey {
				t.Fatalf("envelope=%#v", envelope)
			}
		})
	}
}
