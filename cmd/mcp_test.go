package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestMCPStdioProcess(t *testing.T) {
	if os.Getenv("GOJIRA_MCP_TEST_PROCESS") == "1" {
		if err := newMCPServer(nil, "example.invalid").Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			os.Exit(1)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	process := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMCPStdioProcess$")
	process.Env = append(os.Environ(), "GOJIRA_MCP_TEST_PROCESS=1")
	var diagnostics bytes.Buffer
	process.Stderr = &diagnostics
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: process}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil || len(tools.Tools) != 5 {
		t.Fatalf("stdio tools: %v %v", tools, err)
	}
	guide, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "gojira://guide"})
	if err != nil || len(guide.Contents) != 1 {
		t.Fatalf("stdio guide: %v %v", guide, err)
	}
}

func TestMCPReadOnlyProtocol(t *testing.T) {
	var requests atomic.Int64
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != "GET" && !(r.Method == "POST" && r.URL.Path == "/rest/api/3/search/approximate-count") {
			t.Errorf("unexpected method %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/3/project":
			_, _ = w.Write([]byte(`[{"key":"DEMO","name":"Demo"}]`))
		case "/rest/agile/1.0/board":
			_, _ = w.Write([]byte(`{"values":[],"isLast":true}`))
		case "/rest/agile/1.0/board/1/sprint":
			_, _ = w.Write([]byte(`{"values":[],"isLast":true}`))
		case "/rest/api/3/issue/DEMO-1":
			_, _ = w.Write([]byte(`{"key":"DEMO-1","fields":{"summary":"Example"}}`))
		case "/rest/api/3/search/jql":
			if r.URL.Query().Get("jql") == "truncated" {
				_, _ = w.Write([]byte(`{"issues":[{"key":"DEMO-1","fields":{"summary":"Example"}}],"isLast":false,"nextPageToken":"next"}`))
			} else {
				_, _ = w.Write([]byte(`{"issues":[],"isLast":true}`))
			}
		case "/rest/api/3/search/approximate-count":
			_, _ = w.Write([]byte(`{"count":2}`))
		case "/rest/api/3/issue/AUTH":
			http.Error(w, "private upstream secret", 401)
		case "/rest/api/3/issue/MISSING":
			http.Error(w, "private upstream secret", 404)
		default:
			http.Error(w, "private upstream secret", 403)
		}
	}))
	defer api.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := newMCPServer(jira.NewClient(api.URL, "user", "secret"), "example.atlassian.net")
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	list, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]map[string]any{"gojira_projects": {}, "gojira_boards": {}, "gojira_sprints": {"boardId": 1}, "gojira_get": {"key": "DEMO-1"}, "gojira_search": {"jql": "project = DEMO"}}
	if len(list.Tools) != len(expected) {
		t.Fatalf("tools: %v", list.Tools)
	}
	for _, tool := range list.Tools {
		args, ok := expected[tool.Name]
		if !ok {
			t.Fatalf("unexpected tool %s", tool.Name)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint || !tool.Annotations.IdempotentHint || (tool.Annotations.OpenWorldHint != nil && !*tool.Annotations.OpenWorldHint) {
			t.Fatalf("not read-only: %s", tool.Name)
		}
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool.Name, Arguments: args})
		if err != nil || result.IsError {
			t.Fatalf("%s: %v %v", tool.Name, result, err)
		}
		b, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		var wantData any = []any{}
		switch tool.Name {
		case "gojira_projects":
			wantData = []any{map[string]any{"key": "DEMO", "name": "Demo"}}
		case "gojira_get":
			var issue jira.Issue
			_ = json.Unmarshal([]byte(`{"key":"DEMO-1","fields":{"summary":"Example"}}`), &issue)
			encoded, _ := json.Marshal(jira.IssueToTicketJSON(issue, "example.atlassian.net"))
			_ = json.Unmarshal(encoded, &wantData)
		}
		wantStatus := "unknown"
		if tool.Name == "gojira_search" {
			wantStatus = "complete"
		}
		if got["completeness"] != wantStatus || (tool.Name != "gojira_search" && got["warning"] == "") {
			t.Fatalf("completeness status: %s", b)
		}
		if got["schemaVersion"] != "v1" || got["complete"] != (tool.Name == "gojira_search") || !reflect.DeepEqual(got["data"], wantData) {
			t.Fatalf("%s output contract: %s; want data %#v", tool.Name, b, wantData)
		}
	}
	before := requests.Load()
	if result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_create", Arguments: map[string]any{}}); err == nil {
		t.Fatalf("unknown write tool must fail protocol: %v", result)
	}
	if requests.Load() != before {
		t.Fatal("unknown tool reached Jira")
	}
	for _, tc := range []struct {
		name string
		args any
	}{
		{"gojira_get", map[string]any{}}, {"gojira_get", map[string]any{"key": " "}},
		{"gojira_sprints", map[string]any{"boardId": -1}}, {"gojira_sprints", map[string]any{"boardId": "one"}},
		{"gojira_search", map[string]any{"jql": " "}}, {"gojira_search", map[string]any{"jql": "x", "limit": 1001}},
		{"gojira_projects", map[string]any{"key": "irrelevant"}},
		{"gojira_search", json.RawMessage(`{"jql":"x","limit":0}`)},
		{"gojira_search", json.RawMessage(`{"jql":"x","limit":-0}`)},
		{"gojira_search", json.RawMessage(`{"jql":"x","limit":null}`)},
		{"gojira_search", json.RawMessage(`{"jql":"x","limit":-1}`)},
	} {
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if err != nil || !result.IsError {
			t.Fatalf("invalid input %s: %v %v", tc.name, result, err)
		}
		b, _ := json.Marshal(result.StructuredContent)
		if !strings.Contains(string(b), `"validation_error"`) {
			t.Fatalf("validation contract: %s", b)
		}
	}
	if requests.Load() != before {
		t.Fatal("invalid arguments reached Jira")
	}
	for _, limit := range []int{1, 1000} {
		t.Run(fmt.Sprintf("valid_limit_%d", limit), func(t *testing.T) {
			result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_search", Arguments: map[string]any{"jql": "x", "limit": limit}})
			if err != nil || result.IsError {
				t.Fatalf("boundary: %v %v", result, err)
			}
		})
	}
	for _, key := range []string{"AUTH", "MISSING", "DEMO-2"} {
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_get", Arguments: map[string]any{"key": key}})
		if err != nil || !result.IsError {
			t.Fatalf("HTTP error: %v %v", result, err)
		}
		payload, _ := json.Marshal(result.StructuredContent)
		var failure map[string]any
		_ = json.Unmarshal(payload, &failure)
		wantStatus := map[string]float64{"AUTH": 401, "MISSING": 404, "DEMO-2": 403}[key]
		if failure["httpStatus"] != wantStatus || failure["mutationState"] != "not_applied" || failure["schemaVersion"] != "v1" || failure["code"] == "" {
			t.Fatalf("HTTP contract: %s", payload)
		}
		b, _ := json.Marshal(result)
		if strings.Contains(string(b), "private upstream secret") || strings.Contains(string(b), api.URL) {
			t.Fatalf("unsanitized: %s", b)
		}
	}
	truncated, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_search", Arguments: map[string]any{"jql": "truncated", "limit": 1}})
	if err != nil || truncated.IsError {
		t.Fatalf("truncated search: %v %v", truncated, err)
	}
	tb, _ := json.Marshal(truncated.StructuredContent)
	if !strings.Contains(string(tb), `"complete":false`) || !strings.Contains(string(tb), "results_truncated") || !strings.Contains(string(tb), `"completeness":"partial"`) {
		t.Fatalf("completeness: %s", tb)
	}
	denied, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_get", Arguments: map[string]any{"key": "DEMO-2"}})
	if err != nil || !denied.IsError {
		t.Fatalf("permission failure: %v %v", denied, err)
	}
	b, _ := json.Marshal(denied.StructuredContent)
	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["code"] != "permission_denied" || payload["mutationState"] != "not_applied" {
		t.Fatalf("error contract: %s", b)
	}
	resource, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "gojira://guide"})
	if err != nil || len(resource.Contents) != 1 {
		t.Fatalf("guide: %v %v", resource, err)
	}
}
