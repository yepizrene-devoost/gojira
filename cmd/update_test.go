package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
