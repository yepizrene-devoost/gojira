package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/projectctx"
)

func TestContextJSON(t *testing.T) {
	for _, project := range []string{"ABC", ""} {
		t.Run("project="+project, func(t *testing.T) {
			oldLoad := contextConfig
			contextConfig = func() (*config.Config, error) { return &config.Config{}, nil }
			t.Cleanup(func() { contextConfig = oldLoad })
			t.Setenv("GOJIRA_PROJECT", "")
			t.Setenv("PATH", "")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/rest/api/3/myself" {
					t.Errorf("path=%s", r.URL.Path)
				}
				_, _ = fmt.Fprint(w, `{"accountId":"id","displayName":"Person","active":true,"emailAddress":"private@example.invalid"}`)
			}))
			defer server.Close()
			old := buildClient
			buildClient = func() (*jira.Client, string, error) {
				return jira.NewClient(server.URL, "email", "token"), "domain", nil
			}
			t.Cleanup(func() { buildClient = old })
			cmd := newContextCommand()
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			args := []string{"--json"}
			if project != "" {
				args = append(args, "--project", project)
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if stderr.Len() != 0 || strings.Contains(out.String(), "email") || strings.Contains(out.String(), "token") {
				t.Fatalf("unsafe output")
			}
			var got projectctx.ContextJSON
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.SchemaVersion != "v1" || got.NeedsProject != (project == "") || got.Identity.AccountID != "id" || !got.Identity.Active {
				t.Fatalf("context=%+v", got)
			}
			if project != "" && (got.Project == nil || got.Project.Key != project || got.Project.Source != projectctx.ProjectSourceFlag) {
				t.Fatalf("project=%+v", got.Project)
			}
			if project == "" && got.Project != nil {
				t.Fatal("expected null project")
			}
		})
	}
}

func TestContextErrors(t *testing.T) {
	for _, mode := range []string{"configuration", "invalid", "auth", "network", "writer"} {
		t.Run(mode, func(t *testing.T) {
			oldLoad := contextConfig
			contextConfig = func() (*config.Config, error) { return &config.Config{}, nil }
			t.Cleanup(func() { contextConfig = oldLoad })
			t.Setenv("GOJIRA_PROJECT", "")
			t.Setenv("PATH", "")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "auth" {
					w.WriteHeader(401)
					return
				}
				_, _ = fmt.Fprint(w, `{"accountId":"id"}`)
			}))
			defer server.Close()
			if mode == "network" {
				server.Close()
			}
			old := buildClient
			buildClient = func() (*jira.Client, string, error) {
				if mode == "configuration" {
					return nil, "", errors.New("missing")
				}
				return jira.NewClient(server.URL, "email", "token"), "domain", nil
			}
			t.Cleanup(func() { buildClient = old })
			cmd := newContextCommand()
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			if mode == "writer" {
				cmd.SetOut(failingWriter{err: errors.New("write")})
			}
			args := []string{"--json"}
			if mode == "invalid" {
				args = append(args, "--project", "bad")
			}
			cmd.SetArgs(args)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected error")
			}
			if out.Len() != 0 {
				t.Fatal("stdout on failure")
			}
			var envelope mutationErrorJSON
			if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"configuration": "configuration_error", "invalid": "validation_error", "auth": "authentication_failed", "network": "read_failed", "writer": "output_failed"}[mode]
			if envelope.Code != want {
				t.Fatalf("code=%s want=%s", envelope.Code, want)
			}
		})
	}
}
