package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

type searchReader struct {
	Reader
	limits []int
}

func (r *searchReader) SearchIssues(_ string, limit int) (jira.SearchResult, error) {
	r.limits = append(r.limits, limit)
	return jira.SearchResult{}, nil
}

func TestToolTableValidation(t *testing.T) {
	a := adapter{Options{SchemaVersion: "v1", MapError: func(error) (string, int, string) { return "", 0, "" }}}
	for _, tc := range []struct{ name, raw, message string }{
		{"gojira_projects", `null`, "arguments must be an object"},
		{"gojira_projects", `[]`, "arguments must be an object"},
		{"gojira_projects", `{"x":1}`, "unexpected argument"},
		{"gojira_get", `{"key":null}`, "arguments must not be null"},
		{"gojira_get", `{"key":1}`, "invalid argument type"},
		{"gojira_get", `{}`, "key is required"},
		{"gojira_get", `{"key":" "}`, "key is required"},
		{"gojira_sprints", `{}`, "boardId must be positive"},
		{"gojira_sprints", `{"boardId":0}`, "boardId must be positive"},
		{"gojira_sprints", `{"boardId":-1}`, "boardId must be positive"},
		{"gojira_search", `{}`, "jql is required"},
		{"gojira_search", `{"jql":" "}`, "jql is required"},
		{"gojira_search", `{"jql":"x","limit":0}`, "limit must be between 1 and 1000"},
		{"gojira_search", `{"jql":"x","limit":-0}`, "limit must be between 1 and 1000"},
		{"gojira_search", `{"jql":"x","limit":-1}`, "limit must be between 1 and 1000"},
		{"gojira_search", `{"jql":"x","limit":1001}`, "limit must be between 1 and 1000"},
	} {
		t.Run(tc.name+tc.raw, func(t *testing.T) {
			for _, entry := range a.tools() {
				if entry.name != tc.name {
					continue
				}
				got, err := a.call(entry)(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(tc.raw)}})
				if err != nil || !got.IsError {
					t.Fatalf("validation: %v %v", got, err)
				}
				payload := got.StructuredContent.(map[string]any)
				if payload["message"] != tc.message || payload["code"] != "validation_error" {
					t.Fatalf("payload: %#v", payload)
				}
			}
		})
	}
	// A nil Reader makes any accidental Jira call in the refusal cases fail.
}

func TestSearchDefaultsAndCancellation(t *testing.T) {
	reader := &searchReader{}
	mapped := 0
	a := adapter{Options{Client: reader, SchemaVersion: "v1", MapError: func(err error) (string, int, string) {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("unexpected mapped error: %v", err)
		}
		mapped++
		return "", 0, ""
	}}}
	var search tool
	for _, entry := range a.tools() {
		if entry.name == "gojira_search" {
			search = entry
		}
	}
	for _, raw := range []string{`{"jql":"x"}`, `{"jql":"x","limit":1}`, `{"jql":"x","limit":1000}`} {
		got, err := a.call(search)(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(raw)}})
		if err != nil || got.IsError {
			t.Fatalf("search: %v %v", got, err)
		}
	}
	if !reflect.DeepEqual(reader.limits, []int{50, 1, 1000}) {
		t.Fatalf("limits: %v", reader.limits)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := a.call(search)(ctx, &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{}`)}})
	if err != nil || !got.IsError || mapped != 1 || len(reader.limits) != 3 {
		t.Fatalf("cancellation: %v %v", got, err)
	}
	if got.StructuredContent.(map[string]any)["code"] != "read_failed" {
		t.Fatal("cancellation must precede required JQL validation")
	}
}
