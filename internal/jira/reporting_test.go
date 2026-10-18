package jira_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/reporting"
)

func TestReportingPublic(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/rest/api/3/user/search":
			_, _ = fmt.Fprint(w, `[{"accountId":"a","displayName":"Same","active":true},{"accountId":"b","displayName":"Same","active":true}]`)
		case "/rest/api/3/search/jql":
			_, _ = fmt.Fprint(w, `{"issues":[{"id":"1","key":"P-1"}],"isLast":true}`)
		default:
			_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":2,"worklogs":[{"id":"1","author":{"accountId":"a"},"started":"2026-03-08T23:00:00-0400","timeSpentSeconds":60},{"id":"2","author":{"accountId":"b"},"started":"2026-03-08T23:00:00-0400","timeSpentSeconds":90}]}`)
		}
	}))
	defer server.Close()
	client := jira.NewClient(server.URL, "", "")
	users, err := reporting.SearchUsers(context.Background(), client, "Same")
	if err != nil || len(users.Candidates) != 2 || len(users.Warnings) == 0 {
		t.Fatalf("users: %+v %v", users, err)
	}
	filters := reporting.TimeFilters{AccountID: "a", Project: "P", From: "2026-03-08", To: "2026-03-08", Timezone: "America/New_York"}
	result, err := reporting.TimeReport(context.Background(), client, filters)
	if err != nil || !result.Complete || result.TotalSeconds != 60 {
		t.Fatalf("report: %+v %v", result, err)
	}
	before := requests
	filters.Project = `P" OR project = X`
	if _, err = reporting.TimeReport(context.Background(), client, filters); err == nil || requests != before {
		t.Fatal("invalid scope made request")
	}
}
