package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

type commentReader struct {
	Reader
	key   string
	calls int
}

func (r *commentReader) ListComments(key string) ([]jira.Comment, int, error) {
	r.calls++
	r.key = key
	return []jira.Comment{
		{ID: "10001", Created: "2026-11-09T10:00:00.000+0000", Comment: "first"},
		{ID: "10002", Created: "2026-11-09T11:00:00.000+0000", Comment: "second"},
	}, 2, nil
}

// gojira_comments returns the complete paginated comment history with ids,
// authors and timestamps, marked complete.
func TestCommentsToolReturnsCompleteHistory(t *testing.T) {
	reader := &commentReader{}
	a := adapter{Options{Client: reader, SchemaVersion: "v1"}}
	var comments tool
	found := false
	for _, entry := range a.tools() {
		if entry.name == "gojira_comments" {
			comments, found = entry, true
		}
	}
	if !found {
		t.Fatal("gojira_comments tool is not registered")
	}
	got, err := a.call(comments)(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{"key":"MON-1"}`)}})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got.IsError {
		t.Fatalf("unexpected error result: %#v", got.StructuredContent)
	}
	payload, _ := json.Marshal(got.StructuredContent)
	var out struct {
		SchemaVersion string `json:"schemaVersion"`
		Data          struct {
			Total    int                `json:"total"`
			Comments []jira.CommentJSON `json:"comments"`
		} `json:"data"`
		Complete bool `json:"complete"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("payload=%s err=%v", payload, err)
	}
	if !out.Complete || out.Data.Total != 2 {
		t.Fatalf("payload=%s", payload)
	}
	if len(out.Data.Comments) != 2 || out.Data.Comments[0].ID != "10001" || out.Data.Comments[1].ID != "10002" {
		t.Fatalf("comments=%+v", out.Data.Comments)
	}
	if reader.key != "MON-1" || reader.calls != 1 {
		t.Fatalf("reader key=%q calls=%d", reader.key, reader.calls)
	}
}

// Missing or blank key is rejected as validation_error.
func TestCommentsToolValidatesKey(t *testing.T) {
	a := adapter{Options{SchemaVersion: "v1"}}
	for _, entry := range a.tools() {
		if entry.name != "gojira_comments" {
			continue
		}
		for _, raw := range []string{`{}`, `{"key":" "}`, `{"key":null}`} {
			got, err := a.call(entry)(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(raw)}})
			if err != nil || !got.IsError {
				t.Fatalf("raw=%s: %v %v", raw, got, err)
			}
			payload := got.StructuredContent.(map[string]any)
			if payload["code"] != "validation_error" {
				t.Fatalf("raw=%s payload=%#v", raw, payload)
			}
		}
	}
}
