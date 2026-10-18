package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestReportingCalls(t *testing.T) {
	var requests atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if strings.Contains(r.URL.Path, "user/search") {
			if r.URL.Query().Get("query") == "denied" {
				http.Error(w, "private-upstream-secret", http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`[{"accountId":"a","displayName":"Same","active":true},{"accountId":"b","displayName":"Same","active":true}]`))
			return
		}
		if strings.Contains(r.URL.Path, "search/jql") {
			_, _ = w.Write([]byte(`{"issues":[{"id":"1","key":"DEMO-1"}],"isLast":true}`))
			return
		}
		http.Error(w, "private-upstream-secret", http.StatusForbidden)
	}))
	defer api.Close()
	ctx := context.Background()
	server := New(Options{Client: jira.NewClient(api.URL, "test", "test"), SchemaVersion: "v1", MapError: func(err error) (string, int, string) {
		code, status := jira.ClassifyAPIError(err)
		return code, status, "check access"
	}})
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ss.Close(); err != nil {
			t.Error(err)
		}
	}()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cs.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, tc := range []struct {
		name, args string
		invalid    bool
	}{
		{"gojira_users_search", `{"query":"Same"}`, false},
		{"gojira_users_search", `{"query":"denied"}`, false},
		{"gojira_report_time", `{"accountId":"a","project":"DEMO","from":"2025-01-01","to":"2025-01-01","timezone":"UTC"}`, false},
		{"gojira_users_search", `null`, true},
		{"gojira_users_search", `{"query":null}`, true},
		{"gojira_users_search", `{"query":"Same","unknown":true}`, true},
		{"gojira_report_time", `{"accountId":"a","project":"BAD KEY"}`, true},
		{"gojira_report_time", `{"accountId":"a","project":"DEMO","from":"2025-01-02","to":"2025-01-01","timezone":"Local"}`, true},
	} {
		t.Run(tc.name+tc.args, func(t *testing.T) {
			before := requests.Load()
			value, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: json.RawMessage(tc.args)})
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(value.StructuredContent)
			if strings.Contains(string(encoded), "private-upstream-secret") {
				t.Fatal("leaked upstream details")
			}
			if tc.invalid {
				if !value.IsError || requests.Load() != before || !strings.Contains(string(encoded), "validation_error") {
					t.Fatalf("invalid: %s requests=%d", encoded, requests.Load()-before)
				}
				return
			}
			if strings.Contains(tc.args, "denied") {
				if !value.IsError || !strings.Contains(string(encoded), "403") {
					t.Fatalf("permission: %s", encoded)
				}
				return
			}
			if value.IsError {
				t.Fatalf("unexpected failure: %s", encoded)
			}
			text := value.Content[0].(*mcp.TextContent).Text
			var textValue, structured any
			_ = json.Unmarshal([]byte(text), &textValue)
			_ = json.Unmarshal(encoded, &structured)
			textJSON, _ := json.Marshal(textValue)
			structuredJSON, _ := json.Marshal(structured)
			if string(textJSON) != string(structuredJSON) {
				t.Fatal("text differs from structured result")
			}
			if tc.name == "gojira_report_time" && (!strings.Contains(text, `"complete":false`) || !strings.Contains(text, "partial")) {
				t.Fatal(text)
			}
			if tc.name == "gojira_users_search" && !strings.Contains(text, `"accountId":"b"`) {
				t.Fatal(text)
			}
		})
	}
}
