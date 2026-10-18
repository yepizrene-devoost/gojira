package jira_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/reporting"
)

func TestReportingPages(t *testing.T) {
	for _, mode := range []string{"complete", "failure", "duplicate", "metadata", "invalid-start", "negative"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path == "/rest/api/3/search/jql" {
					if r.URL.Query().Get("nextPageToken") == "next" {
						_, _ = fmt.Fprint(w, `{"issues":[{"id":"2","key":"P-2"}],"isLast":true}`)
					} else {
						_, _ = fmt.Fprint(w, `{"issues":[{"id":"1","key":"P-1"}],"isLast":false,"nextPageToken":"next"}`)
					}
					return
				}
				if r.URL.Path == "/rest/api/3/issue/2/worklog" {
					_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":1,"total":0,"worklogs":[]}`)
					return
				}
				if r.URL.Query().Get("startAt") == "1" {
					switch mode {
					case "failure":
						http.Error(w, "secret body", http.StatusForbidden)
						return
					case "metadata":
						_, _ = fmt.Fprint(w, `{"worklogs":[]}`)
						return
					}
					id, started, seconds := "b", "2026-03-09T00:00:00-0400", 60
					if mode == "duplicate" {
						id = "a"
					}
					if mode == "invalid-start" {
						started = "bad"
					}
					if mode == "negative" {
						seconds = -1
					}
					_, _ = fmt.Fprintf(w, `{"startAt":1,"maxResults":1,"total":2,"worklogs":[{"id":%q,"author":{"accountId":"a"},"started":%q,"timeSpentSeconds":%d}]}`, id, started, seconds)
					return
				}
				_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":1,"total":2,"worklogs":[{"id":"a","author":{"accountId":"a"},"started":"2026-03-08T00:00:00-0500","timeSpentSeconds":120}]}`)
			}))
			defer server.Close()
			client := jira.NewClient(server.URL, "", "")
			f := reporting.TimeFilters{AccountID: "a", Project: "P", From: "2026-03-08", To: "2026-03-08", Timezone: "America/New_York"}
			result, err := reporting.TimeReport(context.Background(), client, f)
			if err != nil || result.TotalSeconds != 120 || result.Complete != (mode == "complete") {
				t.Fatalf("%+v %v", result, err)
			}
			if mode == "failure" {
				warnings := strings.Join(result.Warnings, " ")
				if !strings.Contains(warnings, "HTTP 403") || strings.Contains(warnings, "secret body") {
					t.Fatalf("permission classification missing or leaked: %s", warnings)
				}
			}
			if mode == "complete" && (result.IssuesScanned != 2 || result.WorklogsScanned != 2 || calls != 5) {
				t.Fatalf("scan %+v calls %d", result, calls)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			before := calls
			if _, err := reporting.TimeReport(ctx, client, f); err != context.Canceled || calls != before {
				t.Fatal("cancellation not propagated")
			}
		})
	}
}
