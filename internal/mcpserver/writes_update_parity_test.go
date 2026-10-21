package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func paritySession(t *testing.T, allowWrites bool, handler http.HandlerFunc) *mcp.ClientSession {
	t.Helper()
	api := httptest.NewServer(handler)
	t.Cleanup(api.Close)
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := New(Options{Writer: jira.NewClient(api.URL, "test", "test-token"), AllowWrites: allowWrites}).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "update-regression", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestUpdateSummaryPriorityRegression(t *testing.T) {
	var calls atomic.Int32
	cs := paritySession(t, true, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPut {
			t.Errorf("method=%s", r.Method)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if got := strings.TrimSpace(string(raw)); got != `{"fields":{"priority":{"name":"High"},"summary":" Previous summary "}}` {
			t.Errorf("payload=%s", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "gojira_update", Arguments: map[string]any{"key": "ABC-1", "fields": map[string]any{"summary": " Previous summary ", "priority": "High"}}})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	if res.IsError || string(raw) != `{"action":"update","issueKey":"ABC-1","mutationState":"applied","schemaVersion":"v1"}` || calls.Load() != 1 {
		t.Fatalf("result=%s calls=%d", raw, calls.Load())
	}
}

func TestOtherWriteValidationRegression(t *testing.T) {
	for _, tc := range []struct {
		action  string
		args    map[string]any
		message string
	}{
		{"create", map[string]any{"project": "bad", "issueType": "Task", "summary": "New"}, "invalid project"},
		{"comment", map[string]any{"key": "ABC-1", "body": " "}, "required arguments must not be empty"},
		{"log_work", map[string]any{"key": "ABC-1", "timeSpent": "0h"}, "invalid timeSpent; use positive Jira durations such as 2h 30m"},
		{"assign", map[string]any{"key": "ABC-1", "accountId": " id"}, "accountId must be trimmed"},
		{"move", map[string]any{"key": "ABC-1", "status": 42}, "arguments must be strings"},
		{"delete", map[string]any{"key": "ABC-1", "confirmationToken": " "}, "confirmationToken must not be empty"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			var calls atomic.Int32
			cs := paritySession(t, true, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			})
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "gojira_" + tc.action, Arguments: tc.args})
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(res.StructuredContent)
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if !res.IsError || got["code"] != "validation_error" || got["message"] != tc.message || got["mutationState"] != "not_applied" || got["hint"] != "Correct arguments before retrying." || calls.Load() != 0 {
				t.Fatalf("result=%s calls=%d", raw, calls.Load())
			}
		})
	}
}

func TestUpdateMalformedDescriptionRegression(t *testing.T) {
	for _, fields := range []string{`{"description":42}`, `{"description":null}`, `{"description":"valid","unknown":"value"}`, `{}`} {
		t.Run(fields, func(t *testing.T) {
			var calls atomic.Int32
			cs := paritySession(t, true, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			})
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "gojira_update", Arguments: json.RawMessage(`{"key":"ABC-1","fields":` + fields + `}`)})
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(res.StructuredContent)
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if !res.IsError || got["code"] != "validation_error" || got["mutationState"] != "not_applied" || calls.Load() != 0 {
				t.Fatalf("result=%s calls=%d", raw, calls.Load())
			}
		})
	}
}

func TestReadOnlyDefaultNoWritesRegression(t *testing.T) {
	var calls atomic.Int32
	cs := paritySession(t, false, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	list, err := cs.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range writeTools() {
		for _, tool := range list.Tools {
			if tool.Name == "gojira_"+entry.action {
				t.Errorf("write tool registered: %s", tool.Name)
			}
		}
	}
	if len(list.Tools) != 8 || calls.Load() != 0 {
		t.Fatalf("tools=%d calls=%d", len(list.Tools), calls.Load())
	}
}
