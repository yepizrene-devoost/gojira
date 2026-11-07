package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestAssignPublicContract(t *testing.T) {
	var mu sync.Mutex
	var requests []string
	var bodies []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPut {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			bodies = append(bodies, body["accountId"].(string))
		}
		mu.Unlock()
		switch {
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "ABC-2"):
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("SECRET_MUTATION_RESPONSE"))
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Query().Get("query") == "missing@example.com":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("SECRET_LOOKUP_RESPONSE"))
		case strings.Contains(r.URL.Path, "user/picker"):
			_, _ = w.Write([]byte(`{"users":[{"accountId":"resolved","displayName":"User"}]}`))
		case strings.Contains(r.URL.Path, "/user"):
			_, _ = w.Write([]byte(`{"accountId":"resolved","emailAddress":"person@example.com"}`))
		default:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("SECRET_LOOKUP_RESPONSE"))
		}
	}))
	defer api.Close()
	jiraClient := jira.NewClient(api.URL, "test", "test")
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	server, err := New(Options{AllowWrites: true, Client: jiraClient, Writer: jiraClient}).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	call := func(args map[string]any) *mcp.CallToolResult {
		t.Helper()
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_assign", Arguments: args})
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		return res
	}
	count := func() (int, int) { mu.Lock(); defer mu.Unlock(); return len(requests), len(bodies) }
	for _, tc := range []struct {
		name    string
		args    map[string]any
		account string
	}{
		{"direct account", map[string]any{"key": "ABC-1", "accountId": "direct"}, "direct"},
		{"resolved email", map[string]any{"key": "ABC-1", "email": "person@example.com"}, "resolved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, before := count()
			res := call(tc.args)
			if res.IsError || res.StructuredContent.(map[string]any)["mutationState"] != "applied" {
				t.Fatalf("result=%v", res)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(bodies) != before+1 || bodies[len(bodies)-1] != tc.account {
				t.Fatalf("bodies=%v", bodies)
			}
		})
	}
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"missing", map[string]any{"key": "ABC-1"}},
		{"both", map[string]any{"key": "ABC-1", "email": "person@example.com", "accountId": "direct"}},
		{"empty email", map[string]any{"key": "ABC-1", "email": ""}},
		{"whitespace email", map[string]any{"key": "ABC-1", "email": " person@example.com "}},
		{"empty account", map[string]any{"key": "ABC-1", "accountId": ""}},
		{"null email", map[string]any{"key": "ABC-1", "email": nil}},
		{"numeric email", map[string]any{"key": "ABC-1", "email": 123}},
		{"unexpected field", map[string]any{"key": "ABC-1", "email": "person@example.com", "extra": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			beforeRequests, beforeMutations := count()
			res := call(tc.args)
			payload := res.StructuredContent.(map[string]any)
			if !res.IsError || payload["mutationState"] != "not_applied" {
				t.Fatalf("result=%v", res)
			}
			afterRequests, afterMutations := count()
			if beforeRequests != afterRequests || beforeMutations != afterMutations {
				t.Fatalf("invalid input reached Jira")
			}
		})
	}
	t.Run("unknown mutation outcome is not retried", func(t *testing.T) {
		_, before := count()
		res := call(map[string]any{"key": "ABC-2", "email": "person@example.com"})
		payload := res.StructuredContent.(map[string]any)
		raw, _ := json.Marshal(res)
		if !res.IsError || payload["mutationState"] != "unknown" || strings.Contains(string(raw), "SECRET_MUTATION_RESPONSE") {
			t.Fatalf("result=%s", raw)
		}
		_, after := count()
		if after != before+1 {
			t.Fatalf("mutation requests=%d", after-before)
		}
	})
	t.Run("lookup failure", func(t *testing.T) {
		_, beforeMutations := count()
		res := call(map[string]any{"key": "ABC-1", "email": "missing@example.com"})
		payload := res.StructuredContent.(map[string]any)
		raw, _ := json.Marshal(res)
		if !res.IsError || payload["mutationState"] != "not_applied" || payload["code"] != "lookup_failed" || strings.Contains(string(raw), "SECRET_LOOKUP_RESPONSE") || strings.Contains(string(raw), "missing@example.com") {
			t.Fatalf("result=%s", raw)
		}
		_, afterMutations := count()
		if beforeMutations != afterMutations {
			t.Fatal("mutation after failed lookup")
		}
	})
}
