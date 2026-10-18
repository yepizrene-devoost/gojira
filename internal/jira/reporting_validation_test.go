package jira_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/reporting"
)

func TestReportingInvalidInputsNoRequests(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; http.Error(w, "unexpected", 500) }))
	defer server.Close()
	client := jira.NewClient(server.URL, "", "")
	cases := map[string]func(*reporting.TimeFilters){
		"account":      func(f *reporting.TimeFilters) { f.AccountID = " " },
		"project":      func(f *reporting.TimeFilters) { f.Project = `P OR project=X` },
		"from":         func(f *reporting.TimeFilters) { f.From = "2026-02-30" },
		"to":           func(f *reporting.TimeFilters) { f.To = "tomorrow" },
		"reversed":     func(f *reporting.TimeFilters) { f.To = "2026-03-07" },
		"timezone":     func(f *reporting.TimeFilters) { f.Timezone = "Invalid/Zone" },
		"missing-zone": func(f *reporting.TimeFilters) { f.Timezone = "" },
		"local-zone":   func(f *reporting.TimeFilters) { f.Timezone = "Local" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := reportFilters()
			change(&f)
			if _, err := reporting.TimeReport(context.Background(), client, f); err == nil {
				t.Fatal("invalid filters accepted")
			}
		})
	}
	if _, err := reporting.SearchUsers(context.Background(), client, " "); err == nil {
		t.Fatal("empty query accepted")
	}
	if requests != 0 {
		t.Fatalf("made %d requests", requests)
	}
}
