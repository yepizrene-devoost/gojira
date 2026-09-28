package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestBacklogCLISuccess(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		t.Run(fmt.Sprintf("json=%t", jsonMode), func(t *testing.T) {
			var requests []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.Path)
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/rest/agile/1.0/backlog/issue":
					w.WriteHeader(http.StatusNoContent)
				case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/ARA-12":
					if len(requests) != 2 || !strings.Contains(r.URL.Query().Get("fields"), "description") {
						t.Errorf("refetch = %s, requests = %v", r.URL.String(), requests)
					}
					_, _ = fmt.Fprint(w, `{"key":"ARA-12","fields":{"summary":"Backlog item"}}`)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()
			args := []string{"backlog", "ARA-12"}
			if jsonMode {
				args = append(args, "--json")
			}
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), args...)
			if code != 0 || stderr != "" {
				t.Fatalf("exit=%d stderr=%q", code, stderr)
			}
			want := []string{"POST /rest/agile/1.0/backlog/issue"}
			if jsonMode {
				want = append(want, "GET /rest/api/3/issue/ARA-12")
				var result map[string]any
				assertSingleJSONDocument(t, []byte(stdout), &result)
				if result["schemaVersion"] != "v1" || result["key"] != "ARA-12" || result["summary"] != "Backlog item" {
					t.Fatalf("result = %#v", result)
				}
			} else if stdout != "✓ Jira accepted the request to move ARA-12 to the backlog\n" {
				t.Fatalf("stdout = %q", stdout)
			}
			if fmt.Sprint(requests) != fmt.Sprint(want) {
				t.Fatalf("requests = %v, want %v", requests, want)
			}
		})
	}
}

func TestBacklogInvalidKeyBeforeClientSetup(t *testing.T) {
	for _, key := range []string{"ARA-0", "ara-1", "ARA-1/extra", " ARA-1", "ARA-1?fields=x"} {
		t.Run(key, func(t *testing.T) {
			restoreFlagDefaults(rootCmd)
			oldBuilder, oldOut, oldErr := buildClient, rootCmd.OutOrStdout(), rootCmd.ErrOrStderr()
			var stdout, stderr bytes.Buffer
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stderr)
			builds := 0
			buildClient = func() (*jira.Client, string, error) {
				builds++
				return nil, "", errors.New("client setup must not run")
			}
			t.Cleanup(func() {
				buildClient = oldBuilder
				rootCmd.SetOut(oldOut)
				rootCmd.SetErr(oldErr)
				rootCmd.SetArgs(nil)
				restoreFlagDefaults(rootCmd)
			})
			code := runRootCommand(rootCmd, []string{"backlog", key, "--json"})
			if code != 1 || builds != 0 || stdout.Len() != 0 {
				t.Fatalf("exit=%d builds=%d stdout=%q", code, builds, stdout.String())
			}
			envelope := decodeMutationError(t, stderr.String())
			if envelope.Code != "validation_error" || envelope.MutationState != jira.MutationNotApplied {
				t.Fatalf("envelope = %#v", envelope)
			}
		})
	}
}

func TestBacklogJSONOutputFailureAfterAcceptedWrite(t *testing.T) {
	posts, gets := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		gets++
		_, _ = fmt.Fprint(w, `{"key":"ARA-12","fields":{"summary":"Backlog item"}}`)
	}))
	defer srv.Close()
	restoreFlagDefaults(rootCmd)
	oldBuilder, oldOut, oldErr := buildClient, rootCmd.OutOrStdout(), rootCmd.ErrOrStderr()
	var stderr bytes.Buffer
	rootCmd.SetOut(failingWriter{err: errors.New("stdout unavailable")})
	rootCmd.SetErr(&stderr)
	buildClient = func() (*jira.Client, string, error) {
		return jira.NewClient(srv.URL, "email", "token"), "jira.example", nil
	}
	t.Cleanup(func() {
		buildClient = oldBuilder
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
		rootCmd.SetArgs(nil)
		restoreFlagDefaults(rootCmd)
	})
	if code := runRootCommand(rootCmd, []string{"backlog", "ARA-12", "--json"}); code != 1 || posts != 1 || gets != 1 {
		t.Fatalf("exit=%d posts=%d gets=%d", code, posts, gets)
	}
	if envelope := decodeMutationError(t, stderr.String()); envelope.Code != "output_failed" || envelope.MutationState != jira.MutationApplied {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestBacklogJSONFailureStates(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		wantCode  string
		wantState jira.MutationState
	}{
		{"rejected", http.StatusBadRequest, "mutation_failed", jira.MutationNotApplied},
		{"server error", http.StatusBadGateway, "mutation_failed", jira.MutationUnknown},
		{"refetch failed", http.StatusNoContent, "refetch_failed", jira.MutationApplied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts, gets := 0, 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
					w.WriteHeader(tc.status)
				} else {
					gets++
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer srv.Close()
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "backlog", "ARA-12", "--json")
			if code != 1 || stdout != "" || posts != 1 || (gets == 1) != (tc.status == http.StatusNoContent) {
				t.Fatalf("exit=%d stdout=%q posts=%d gets=%d", code, stdout, posts, gets)
			}
			envelope := decodeMutationError(t, stderr)
			if envelope.Code != tc.wantCode || envelope.MutationState != tc.wantState || envelope.IssueKey != "ARA-12" {
				t.Fatalf("envelope = %#v", envelope)
			}
		})
	}
	t.Run("lost response", func(t *testing.T) {
		requests := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.Close()
		}))
		defer srv.Close()
		code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), "backlog", "ARA-12", "--json")
		if code != 1 || stdout != "" || requests != 1 {
			t.Fatalf("exit=%d stdout=%q requests=%d", code, stdout, requests)
		}
		if envelope := decodeMutationError(t, stderr); envelope.Code != "mutation_failed" || envelope.MutationState != jira.MutationUnknown {
			t.Fatalf("envelope = %#v", envelope)
		}
	})
}
