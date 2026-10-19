package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestMCPWrites(t *testing.T) {
	for _, status := range []int{200, 400, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var mutations atomic.Int32
			var lastRequest atomic.Value
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte(`{"transitions":[{"id":"1","name":"Done"}]}`))
					return
				}
				body, _ := io.ReadAll(r.Body)
				lastRequest.Store(r.Method + " " + r.URL.Path + " " + string(body))
				mutations.Add(1)
				w.WriteHeader(status)
				if status == 200 {
					_, _ = w.Write([]byte(`{"key":"DEMO-1"}`))
				} else {
					_, _ = w.Write([]byte(`SECRET_MARKER https://private.invalid`))
				}
			}))
			defer api.Close()
			ctx := context.Background()
			st, ct := mcp.NewInMemoryTransports()
			ss, err := newMCPServer(jira.NewClient(api.URL, "user", "secret"), "example.invalid", true).Connect(ctx, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = ss.Close() }()
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "writes-test", Version: "1"}, nil).Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cs.Close() }()
			list, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
			if err != nil {
				t.Fatal(err)
			}
			if len(list.Tools) != 15 {
				t.Fatalf("tools=%d", len(list.Tools))
			}
			for _, tool := range list.Tools {
				if tool.Name == "gojira_delete" && (tool.Annotations.ReadOnlyHint || !*tool.Annotations.DestructiveHint || tool.Annotations.IdempotentHint) {
					t.Fatal("delete annotations")
				}
			}
			for _, test := range []struct {
				name string
				args map[string]any
			}{
				{"create", map[string]any{"project": "DEMO", "issueType": "Task", "summary": "Example"}},
				{"comment", map[string]any{"key": "DEMO-1", "body": "Example"}},
				{"log_work", map[string]any{"key": "DEMO-1", "timeSpent": "2h"}},
				{"assign", map[string]any{"key": "DEMO-1", "accountId": "id"}},
				{"move", map[string]any{"key": "DEMO-1", "status": "Done"}},
				{"update", map[string]any{"key": "DEMO-1", "fields": map[string]any{"summary": "Example"}}},
				{"delete", map[string]any{"key": "DEMO-1"}},
			} {
				before := mutations.Load()
				if test.name == "delete" {
					preview, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_delete", Arguments: test.args})
					if e != nil {
						t.Fatal(e)
					}
					raw, _ := json.Marshal(preview.StructuredContent)
					var value struct {
						Data struct {
							Token string `json:"confirmationToken"`
						} `json:"data"`
					}
					if e = json.Unmarshal(raw, &value); e != nil || value.Data.Token == "" {
						t.Fatalf("preview %s %v", raw, e)
					}
					if mutations.Load() != before {
						t.Fatal("preview mutated")
					}
					test.args["confirmationToken"] = value.Data.Token
				}
				res, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_" + test.name, Arguments: test.args})
				if e != nil {
					t.Fatal(e)
				}
				raw, _ := json.Marshal(res.StructuredContent)
				want := "applied"
				if status == 400 {
					want = "not_applied"
				}
				if status == 500 {
					want = "unknown"
				}
				if !strings.Contains(string(raw), `"mutationState":"`+want+`"`) || strings.Contains(string(raw), "SECRET_MARKER") || strings.Contains(string(raw), "private.invalid") {
					t.Fatalf("%s: %s", test.name, raw)
				}
				if mutations.Load() != before+1 {
					t.Fatalf("%s request count", test.name)
				}
				expected := map[string]string{
					"create":   "POST /rest/api/3/issue ",
					"comment":  "POST /rest/api/3/issue/DEMO-1/comment ",
					"log_work": "POST /rest/api/3/issue/DEMO-1/worklog ",
					"assign":   "PUT /rest/api/3/issue/DEMO-1/assignee ",
					"move":     "POST /rest/api/3/issue/DEMO-1/transitions ",
					"update":   "PUT /rest/api/3/issue/DEMO-1 ",
					"delete":   "DELETE /rest/api/3/issue/DEMO-1 ",
				}
				request := lastRequest.Load().(string)
				if !strings.HasPrefix(request, expected[test.name]) {
					t.Fatalf("%s request: %s", test.name, request)
				}
				if test.name == "delete" {
					reused, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_delete", Arguments: test.args})
					if e != nil || !reused.IsError || mutations.Load() != before+1 {
						t.Fatalf("reused token: %v %v", reused, e)
					}
				}
			}
			before := mutations.Load()
			for _, args := range []map[string]any{{"key": "DEMO-1", "accountId": " id "}, {"key": "DEMO-1", "accountId": nil}, {"key": "DEMO-1", "accountId": 1}, {"key": "DEMO-1", "accountId": "id", "extra": true}} {
				res, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_assign", Arguments: args})
				if e == nil && !res.IsError {
					t.Fatal("invalid accepted")
				}
			}
			if mutations.Load() != before {
				t.Fatal("invalid input mutated")
			}
		})
	}
}
