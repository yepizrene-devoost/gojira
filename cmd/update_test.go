package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestUpdateDescriptionPayloadPreservesOtherRequestedFields(t *testing.T) {
	var fields map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/rest/api/3/issue/A-1" {
			t.Errorf("request = %s %s, want PUT issue", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var payload struct {
			Fields map[string]json.RawMessage `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fields = payload.Fields
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"),
		"update", "A-1",
		"--description", "First paragraph\n- one",
		"--summary", "Kept summary",
		"--priority", "High",
		"--labels", "one, two")
	if code != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if stdout != "✓ Updated A-1: description, labels, priority, summary\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	for _, name := range []string{"description", "summary", "priority", "labels"} {
		if _, ok := fields[name]; !ok {
			t.Fatalf("payload omitted %q: %#v", name, fields)
		}
	}
	var description jira.ADFDoc
	if err := json.Unmarshal(fields["description"], &description); err != nil {
		t.Fatalf("decode description: %v", err)
	}
	if got := description.Flatten(); got != "First paragraph\n- one" {
		t.Fatalf("description = %q", got)
	}
}

func TestUpdateExplicitEmptyDescriptionClearsField(t *testing.T) {
	var description json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Fields map[string]json.RawMessage `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		description = payload.Fields["description"]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"),
		"update", "A-1", "--description=")
	if code != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if stdout != "✓ Updated A-1: description\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if string(description) != "null" {
		t.Fatalf("description payload = %s, want null", description)
	}
}

func TestUpdateDescriptionJSONReturnsResultingTicket(t *testing.T) {
	mutationSeen := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			mutationSeen = true
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			if !mutationSeen {
				t.Error("result fetched before mutation")
			}
			_, _ = fmt.Fprint(w, `{"key":"A-1","fields":{"summary":"Result","description":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Updated context"}]}]}}}`)
		default:
			t.Errorf("unexpected method %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"),
		"update", "A-1", "--description", "Updated context", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	var ticket jira.TicketJSON
	assertSingleJSONDocument(t, []byte(stdout), &ticket)
	if ticket.SchemaVersion != "v1" || ticket.Key != "A-1" || ticket.Description != "Updated context" {
		t.Fatalf("ticket = %#v", ticket)
	}
}

func TestUpdateDateComponentsAndFixVersionsUsesEditMetaThenReturnsJSON(t *testing.T) {
	var sequence []string
	var fields map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sequence = append(sequence, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/editmeta"):
			_, _ = fmt.Fprint(w, `{"fields":{"duedate":{},"components":{"allowedValues":[{"name":"API"},{"name":"Web"}]},"fixVersions":{}}}`)
		case r.Method == http.MethodPut:
			var payload struct {
				Fields map[string]json.RawMessage `json:"fields"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode payload: %v", err)
			}
			fields = payload.Fields
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet:
			_, _ = fmt.Fprint(w, `{"key":"A-1","fields":{"summary":"Updated","duedate":"2026-10-31","components":[{"name":"API"},{"name":"Web"}],"fixVersions":[{"name":"v1.0"},{"name":"v1.1"}]}}`)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"),
		"update", "A-1", "--summary", "Updated", "--due-date", "2026-10-31",
		"--components", "API, Web", "--fix-versions", "v1.0,v1.1", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	wantSequence := []string{"GET /rest/api/3/issue/A-1/editmeta", "PUT /rest/api/3/issue/A-1", "GET /rest/api/3/issue/A-1"}
	if fmt.Sprint(sequence) != fmt.Sprint(wantSequence) {
		t.Fatalf("request sequence = %v, want %v", sequence, wantSequence)
	}
	if string(fields["duedate"]) != `"2026-10-31"` || string(fields["components"]) != `[{"name":"API"},{"name":"Web"}]` || string(fields["fixVersions"]) != `[{"name":"v1.0"},{"name":"v1.1"}]` {
		t.Fatalf("fields payload = %s", mustJSON(t, fields))
	}
	var ticket jira.TicketJSON
	assertSingleJSONDocument(t, []byte(stdout), &ticket)
	if ticket.DueDate == nil || *ticket.DueDate != "2026-10-31" || fmt.Sprint(ticket.Components) != "[API Web]" || fmt.Sprint(ticket.FixVersions) != "[v1.0 v1.1]" {
		t.Fatalf("ticket = %#v", ticket)
	}
}

func TestUpdateExplicitEmptyNewFieldsClearsThem(t *testing.T) {
	var fields map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, `{"fields":{"duedate":{},"components":{"allowedValues":[]},"fixVersions":{"allowedValues":[]}}}`)
			return
		}
		var payload struct {
			Fields map[string]json.RawMessage `json:"fields"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		fields = payload.Fields
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"),
		"update", "A-1", "--due-date=", "--components=", "--fix-versions=")
	if code != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if stdout != "✓ Updated A-1: components, duedate, fixVersions\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if string(fields["duedate"]) != "null" || string(fields["components"]) != "[]" || string(fields["fixVersions"]) != "[]" {
		t.Fatalf("clear payload = %s", mustJSON(t, fields))
	}
}

func TestUpdateRejectsInvalidNewFieldInputsBeforeAnyRequest(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "invalid calendar date", args: []string{"--due-date", "2026-02-30"}},
		{name: "date has time", args: []string{"--due-date", "2026-10-31T00:00:00Z"}},
		{name: "component empty entry", args: []string{"--components", "API,,Web"}},
		{name: "component duplicate ignoring case", args: []string{"--components", "API,api"}},
		{name: "version trailing entry", args: []string{"--fix-versions", "v1.0,"}},
		{name: "version duplicate", args: []string{"--fix-versions", "v1.0,v1.0"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()
			args := append([]string{"update", "A-1"}, tc.args...)
			args = append(args, "--json")
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), args...)
			if code != 1 || stdout != "" || requests != 0 {
				t.Fatalf("exit = %d, stdout = %q, requests = %d", code, stdout, requests)
			}
			var envelope map[string]any
			assertSingleJSONDocument(t, []byte(stderr), &envelope)
			if envelope["code"] != "validation_error" || envelope["mutationState"] != "not_applied" || envelope["issueKey"] != "A-1" {
				t.Fatalf("error envelope = %#v", envelope)
			}
		})
	}
}

func TestUpdateLetsJiraRejectNameWhenEditMetaOmitsAllowedValues(t *testing.T) {
	var puts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = fmt.Fprint(w, `{"fields":{"components":{}}}`)
		case http.MethodPut:
			puts++
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"errors":{"components":"unknown component"}}`)
		}
	}))
	defer srv.Close()

	code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"),
		"update", "A-1", "--components", "Unknown", "--json")
	if code != 1 || stdout != "" || puts != 1 {
		t.Fatalf("exit = %d, stdout = %q, puts = %d", code, stdout, puts)
	}
	var envelope map[string]any
	assertSingleJSONDocument(t, []byte(stderr), &envelope)
	if envelope["code"] != "mutation_failed" || envelope["mutationState"] != "not_applied" {
		t.Fatalf("error envelope = %#v", envelope)
	}
}

func TestUpdateFailsSafelyWhenEditMetadataRejectsRequestedFields(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		metadata    string
		args        []string
		wantCode    string
		wantMessage string
	}{
		{name: "metadata unavailable", status: http.StatusForbidden, args: []string{"--due-date", "2026-10-31"}, wantCode: "metadata_failed", wantMessage: "issue edit metadata is unavailable"},
		{name: "field absent", status: http.StatusOK, metadata: `{"fields":{"components":{}}}`, args: []string{"--fix-versions", "v1.0"}, wantCode: "validation_error", wantMessage: "requested field update is not allowed"},
		{name: "component not allowed", status: http.StatusOK, metadata: `{"fields":{"components":{"allowedValues":[{"name":"Web"}]}}}`, args: []string{"--components", "API"}, wantCode: "validation_error", wantMessage: "requested field update is not allowed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			puts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPut {
					puts++
				}
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.metadata)
			}))
			defer srv.Close()
			args := append([]string{"update", "A-1"}, tc.args...)
			args = append(args, "--json")
			code, stdout, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), args...)
			if code != 1 || stdout != "" || puts != 0 {
				t.Fatalf("exit = %d, stdout = %q, puts = %d", code, stdout, puts)
			}
			var envelope map[string]any
			assertSingleJSONDocument(t, []byte(stderr), &envelope)
			if envelope["code"] != tc.wantCode || envelope["message"] != tc.wantMessage || envelope["mutationState"] != "not_applied" {
				t.Fatalf("error envelope = %#v", envelope)
			}
		})
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
