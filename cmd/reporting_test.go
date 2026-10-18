package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestReportingInvalidBeforeClient(t *testing.T) {
	old := buildClient
	defer func() { buildClient = old }()
	calls := 0
	buildClient = func() (*jira.Client, string, error) { calls++; return nil, "", nil }
	for _, args := range [][]string{{"users", "search", " "}, {"report", "time", "--account-id", "id", "--project", "BAD KEY", "--from", "2025-01-01", "--to", "2025-01-02", "--timezone", "UTC"}} {
		t.Run(args[0], func(t *testing.T) {
			root := &cobra.Command{Use: "gojira", SilenceErrors: true, SilenceUsage: true}
			root.AddCommand(newUsersCommand(), newReportCommand())
			var out, stderr bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&stderr)
			root.SetArgs(append(args, "--json"))
			if err := root.Execute(); err == nil {
				t.Fatal("expected validation failure")
			}
			if out.Len() != 0 {
				t.Fatal(out.String())
			}
		})
	}
	if calls != 0 {
		t.Fatalf("client builds=%d", calls)
	}
}

func TestReportingPublicCLI(t *testing.T) {
	old := buildClient
	defer func() { buildClient = old }()
	for _, scenario := range []string{"users", "time", "partial", "401", "403", "canceled", "writer"} {
		t.Run(scenario, func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if scenario == "401" || scenario == "403" {
					status := 401
					if scenario == "403" {
						status = 403
					}
					http.Error(w, "private-upstream-secret", status)
					return
				}
				if strings.Contains(r.URL.Path, "user/search") {
					_, _ = w.Write([]byte(`[{"accountId":"a","displayName":"Same","active":true},{"accountId":"b","displayName":"Same","active":true}]`))
					return
				}
				if strings.Contains(r.URL.Path, "search/jql") {
					_, _ = w.Write([]byte(`{"issues":[{"id":"1","key":"DEMO-1"}],"isLast":true}`))
					return
				}
				if scenario == "partial" {
					http.Error(w, "private-upstream-secret", http.StatusForbidden)
					return
				}
				_, _ = w.Write([]byte(`{"startAt":0,"maxResults":100,"total":2,"worklogs":[{"id":"1","author":{"accountId":"a"},"started":"2025-01-01T12:00:00Z","timeSpentSeconds":60},{"id":"2","author":{"accountId":"b"},"started":"2025-01-01T12:00:00Z","timeSpentSeconds":99}]}`))
			}))
			defer api.Close()
			buildClient = func() (*jira.Client, string, error) { return jira.NewClient(api.URL, "test", "test"), "", nil }
			root := &cobra.Command{Use: "gojira", SilenceErrors: true, SilenceUsage: true}
			root.AddCommand(newUsersCommand(), newReportCommand())
			var out, stderr bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&stderr)
			args := []string{"report", "time", "--account-id", "a", "--project", "DEMO", "--from", "2025-01-01", "--to", "2025-01-01", "--timezone", "UTC", "--json"}
			if scenario == "users" {
				args = []string{"users", "search", "Same", "--json"}
			}
			if scenario == "partial" {
				args = args[:len(args)-1]
			}
			if scenario == "writer" {
				root.SetOut(reportingBrokenWriter{})
			}
			if scenario == "canceled" {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				root.SetContext(ctx)
			}
			root.SetArgs(args)
			err := root.Execute()
			if scenario == "401" || scenario == "403" || scenario == "canceled" || scenario == "writer" {
				if err == nil {
					t.Fatal("expected failure")
				}
				if strings.Contains(out.String()+stderr.String()+err.Error(), "private-upstream-secret") {
					t.Fatal("leaked upstream error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "partial" {
				if !strings.Contains(out.String(), "PARTIAL") || !strings.Contains(out.String(), "WARNING:") {
					t.Fatal(out.String())
				}
				return
			}
			var value map[string]any
			decoder := json.NewDecoder(&out)
			if err := decoder.Decode(&value); err != nil {
				t.Fatal(err)
			}
			if decoder.Decode(new(any)) == nil {
				t.Fatal("multiple JSON results")
			}
			if scenario == "users" {
				if len(value["candidates"].([]any)) != 2 {
					t.Fatal(value)
				}
			} else if value["totalSeconds"] != float64(60) || value["complete"] != true {
				t.Fatal(value)
			}
		})
	}
}

type reportingBrokenWriter struct{}

func (reportingBrokenWriter) Write([]byte) (int, error) { return 0, errors.New("writer failed") }
