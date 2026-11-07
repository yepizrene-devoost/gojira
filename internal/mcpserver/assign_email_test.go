package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestAssignEmailPublic(t *testing.T) {
	var requests []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch {
		case strings.Contains(r.URL.Path, "user/picker"):
			w.Write([]byte(`{"users":[{"accountId":"resolved","displayName":"User"}]}`))
		case r.Method == "GET":
			w.Write([]byte(`{"accountId":"resolved","emailAddress":"person@example.com"}`))
		default:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["accountId"] != "resolved" {
				t.Errorf("body=%v", body)
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer api.Close()
	client := jira.NewClient(api.URL, "test", "test")
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	server, err := New(Options{AllowWrites: true, Writer: client, Client: client}).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range tools.Tools {
		if tool.Name == "gojira_assign" {
			found = true
			encoded, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			var schema struct {
				Required   []string                   `json:"required"`
				Properties map[string]json.RawMessage `json:"properties"`
				OneOf      []struct {
					Required []string `json:"required"`
				} `json:"oneOf"`
			}
			if err := json.Unmarshal(encoded, &schema); err != nil {
				t.Fatal(err)
			}
			if len(schema.Required) != 1 || schema.Required[0] != "key" || len(schema.Properties["email"]) == 0 || len(schema.Properties["accountId"]) == 0 || len(schema.OneOf) != 2 || len(schema.OneOf[0].Required) != 1 || schema.OneOf[0].Required[0] != "accountId" || len(schema.OneOf[1].Required) != 1 || schema.OneOf[1].Required[0] != "email" {
				t.Fatalf("schema=%s", encoded)
			}
		}
	}
	if !found {
		t.Fatal("gojira_assign not advertised")
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_assign", Arguments: map[string]any{"key": "ABC-1", "email": "person@example.com"}})
	if err != nil || result.IsError {
		t.Fatalf("result=%v err=%v requests=%v", result, err, requests)
	}
	if len(requests) != 3 || requests[2] != "PUT /rest/api/3/issue/ABC-1/assignee" {
		t.Fatalf("requests=%v", requests)
	}
}
