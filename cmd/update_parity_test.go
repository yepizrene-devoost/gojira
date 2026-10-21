package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// These cases are the unchanged CLI baseline for the parent-owned differential.
func TestUpdateParityCLIBaseline(t *testing.T) {
	const pointsField = "customfield_10016"
	tests := []struct {
		name                      string
		flags                     []string
		payload, updated, failure string
		rejectMeta                bool
	}{
		{name: "summary", flags: []string{"--summary", "New"}, payload: `{"fields":{"summary":"New"}}`, updated: "summary"},
		{name: "description", flags: []string{"--description", "Context"}, payload: `{"fields":{"description":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Context"}]}]}}}`, updated: "description"},
		{name: "description_clear", flags: []string{"--description="}, payload: `{"fields":{"description":null}}`, updated: "description"},
		{name: "priority", flags: []string{"--priority", "High"}, payload: `{"fields":{"priority":{"name":"High"}}}`, updated: "priority"},
		{name: "labels", flags: []string{"--labels", "one, two"}, payload: `{"fields":{"labels":["one","two"]}}`, updated: "labels"},
		{name: "due_date", flags: []string{"--due-date", "2028-02-29"}, payload: `{"fields":{"duedate":"2028-02-29"}}`, updated: "duedate"},
		{name: "due_date_clear", flags: []string{"--due-date="}, payload: `{"fields":{"duedate":null}}`, updated: "duedate"},
		{name: "due_date_invalid", flags: []string{"--due-date", "2027-02-29"}, failure: "--due-date must be a real calendar date in YYYY-MM-DD format"},
		{name: "components", flags: []string{"--components", "API, Web"}, payload: `{"fields":{"components":[{"name":"API"},{"name":"Web"}]}}`, updated: "components"},
		{name: "components_clear", flags: []string{"--components="}, payload: `{"fields":{"components":[]}}`, updated: "components"},
		{name: "components_duplicate", flags: []string{"--components", "API,api"}, failure: `--components contains duplicate entry "api"`},
		{name: "components_empty_entry", flags: []string{"--components", "API,"}, failure: "--components contains an empty entry"},
		{name: "fix_versions", flags: []string{"--fix-versions", "v1, v2"}, payload: `{"fields":{"fixVersions":[{"name":"v1"},{"name":"v2"}]}}`, updated: "fixVersions"},
		{name: "fix_versions_clear", flags: []string{"--fix-versions="}, payload: `{"fields":{"fixVersions":[]}}`, updated: "fixVersions"},
		{name: "story_points", flags: []string{"--story-points", "9007199254740993.125", "--story-points-field", pointsField}, payload: `{"fields":{"customfield_10016":9007199254740993.125}}`, updated: pointsField},
		{name: "story_points_clear", flags: []string{"--story-points=", "--story-points-field", pointsField}, payload: `{"fields":{"customfield_10016":null}}`, updated: pointsField},
		{name: "story_points_invalid", flags: []string{"--story-points", "1e3", "--story-points-field", pointsField}, failure: "--story-points must be a finite decimal number or explicitly empty"},
		{name: "story_points_unpaired", flags: []string{"--story-points", "5"}, failure: "--story-points and --story-points-field must be provided together"},
		{name: "story_points_field_unpaired", flags: []string{"--story-points-field", pointsField}, failure: "--story-points and --story-points-field must be provided together"},
		{name: "metadata_rejection", flags: []string{"--components", "API"}, failure: `field "components" is not editable for this issue`, rejectMeta: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var payload string
			var methods []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				methods = append(methods, r.Method)
				if r.Method == http.MethodGet {
					if tc.rejectMeta {
						_, _ = fmt.Fprint(w, `{"fields":{}}`)
						return
					}
					_, _ = fmt.Fprint(w, `{"fields":{"duedate":{},"components":{},"fixVersions":{},"customfield_10016":{"schema":{"type":"number"},"operations":["set"]}}}`)
					return
				}
				if r.Method != http.MethodPut {
					t.Errorf("unexpected method %s", r.Method)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				payload = strings.TrimSpace(string(body))
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()
			t.Setenv("JIRA_DOMAIN", srv.URL)
			t.Setenv("JIRA_EMAIL", "test@example.com")
			t.Setenv("JIRA_API_TOKEN", "test-token")
			// Injecting the client also prevents BuildClient's dotenv/config loading.
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "test@example.com", "test-token"), append([]string{"update", "A-1"}, tc.flags...)...)
			if tc.failure != "" {
				if code != 1 || stdout != "" || stderr != "Error: "+tc.failure+"\n" || payload != "" {
					t.Fatalf("exit=%d stdout=%q stderr=%q payload=%q", code, stdout, stderr, payload)
				}
				wantCalls := 0
				if tc.rejectMeta {
					wantCalls = 1
				}
				if len(methods) != wantCalls {
					t.Fatalf("requests=%v, want %d", methods, wantCalls)
				}
				return
			}
			if code != 0 || stderr != "" || stdout != "✓ Updated A-1: "+tc.updated+"\n" {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if payload != tc.payload {
				t.Fatalf("payload=%s, want %s", payload, tc.payload)
			}
			wantMethods := "PUT"
			if tc.updated == "duedate" || tc.updated == "components" || tc.updated == "fixVersions" || tc.updated == pointsField {
				wantMethods = "GET PUT"
			}
			if strings.Join(methods, " ") != wantMethods {
				t.Fatalf("requests=%v, want %s", methods, wantMethods)
			}
		})
	}
}
