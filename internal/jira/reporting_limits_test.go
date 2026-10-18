package jira_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/reporting"
)

func TestReportingUserPagination(t *testing.T) {
	for _, mode := range []string{"exhausted", "cap", "failure", "duplicate", "permission-empty"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				offset, _ := strconv.Atoi(r.URL.Query().Get("startAt"))
				if offset != calls*100 {
					t.Errorf("offset %d calls %d", offset, calls)
				}
				calls++
				if mode == "failure" && offset == 100 {
					http.Error(w, "private", http.StatusForbidden)
					return
				}
				count := 100
				if mode == "exhausted" && offset == 100 {
					count = 2
				}
				if mode == "permission-empty" {
					count = 0
				}
				users := make([]reporting.User, 0, count)
				for i := 0; i < count; i++ {
					id := offset + i
					if mode == "duplicate" {
						id = i
					}
					users = append(users, reporting.User{AccountID: fmt.Sprint(id), DisplayName: "Same", Active: true})
				}
				_ = json.NewEncoder(w).Encode(users)
			}))
			defer server.Close()
			result, err := reporting.SearchUsers(context.Background(), jira.NewClient(server.URL, "", ""), "Same")
			if err != nil || len(result.Warnings) == 0 {
				t.Fatalf("%+v %v", result, err)
			}
			switch mode {
			case "cap":
				if calls != 10 || len(result.Candidates) != 1000 || result.Completeness != "partial" {
					t.Fatalf("%+v calls %d", result, calls)
				}
			case "exhausted":
				if calls != 2 || len(result.Candidates) != 102 || result.Completeness != "endpoint_window_exhausted" {
					t.Fatalf("%+v", result)
				}
			case "permission-empty":
				if calls != 1 || len(result.Candidates) != 0 {
					t.Fatalf("%+v", result)
				}
			default:
				if calls != 2 || len(result.Candidates) != 100 || result.Completeness != "partial" {
					t.Fatalf("%+v", result)
				}
			}
		})
	}
}

func TestReportingLimits(t *testing.T) {
	for _, mode := range []string{"issue-cap", "exact-issue-cap", "budget", "overflow"} {
		t.Run(mode, func(t *testing.T) {
			calls, issuePages := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path == "/rest/api/3/search/jql" {
					issuePages++
					issues := []reporting.Issue{}
					count := 100
					if mode == "budget" || mode == "overflow" {
						count = 1
					}
					for i := 0; i < count; i++ {
						id := (issuePages-1)*100 + i
						issues = append(issues, reporting.Issue{ID: fmt.Sprint(id), Key: fmt.Sprintf("P-%04d", id)})
					}
					last := mode == "budget" || mode == "overflow" || (mode == "exact-issue-cap" && issuePages == 10)
					_ = json.NewEncoder(w).Encode(map[string]any{"issues": issues, "isLast": last, "nextPageToken": fmt.Sprint(issuePages)})
					return
				}
				if mode == "budget" {
					offset, _ := strconv.Atoi(r.URL.Query().Get("startAt"))
					_, _ = fmt.Fprintf(w, `{"startAt":%d,"maxResults":1,"total":3000,"worklogs":[{"id":"w%d","author":{"accountId":"a"},"started":"2026-03-08T12:00:00Z","timeSpentSeconds":1}]}`, offset, offset)
					return
				}
				if mode == "overflow" {
					_, _ = fmt.Fprintf(w, `{"startAt":0,"maxResults":100,"total":2,"worklogs":[{"id":"a","author":{"accountId":"a"},"started":"2026-03-08T12:00:00Z","timeSpentSeconds":%d},{"id":"b","author":{"accountId":"a"},"started":"2026-03-08T12:00:00Z","timeSpentSeconds":1}]}`, int64(math.MaxInt64))
					return
				}
				_, _ = fmt.Fprint(w, `{"startAt":0,"maxResults":100,"total":0,"worklogs":[]}`)
			}))
			defer server.Close()
			result, err := reporting.TimeReport(context.Background(), jira.NewClient(server.URL, "", ""), reportFilters())
			if err != nil || result.PageRequests != calls {
				t.Fatalf("%+v calls %d err %v", result, calls, err)
			}
			if mode == "exact-issue-cap" {
				if !result.Complete || result.IssuesScanned != 1000 || calls != 1010 {
					t.Fatalf("%+v", result)
				}
				return
			}
			if result.Complete {
				t.Fatal("limit returned complete")
			}
			warnings := strings.Join(result.Warnings, " ")
			switch mode {
			case "issue-cap":
				if result.IssuesScanned != 1000 || calls != 1011 || !strings.Contains(warnings, "cap reached") {
					t.Fatalf("%+v calls %d", result, calls)
				}
			case "budget":
				if calls != 2000 || result.WorklogsScanned != 1999 || result.TotalSeconds != 1999 || !strings.Contains(warnings, "budget reached") {
					t.Fatalf("%+v calls %d", result, calls)
				}
			case "overflow":
				if result.TotalSeconds != math.MaxInt64 || len(result.Rows) != 1 || result.Rows[0].Seconds != math.MaxInt64 || !strings.Contains(warnings, "overflow") {
					t.Fatalf("%+v", result)
				}
			}
		})
	}
}
