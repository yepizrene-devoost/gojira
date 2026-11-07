package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// writerWithoutSprintLookup exposes only the mutation interface.
type writerWithoutSprintLookup struct{ Writer }

func TestCreateBoardMissingSprintLookup(t *testing.T) {
	var mu sync.Mutex
	var calls int
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/issue" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"key":"ABC-1"}`)
	}))
	defer api.Close()
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	writer := writerWithoutSprintLookup{jira.NewClient(api.URL, "user", "secret")}
	ss, err := New(Options{AllowWrites: true, Writer: writer}).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_create", Arguments: map[string]any{"project": "ABC", "issueType": "Task", "summary": "Example", "boardId": 7}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(res.StructuredContent)
	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatal(err)
	}
	if !res.IsError || payload["code"] != "partial_failure" || payload["mutationState"] != "applied" || payload["issueKey"] != "ABC-1" {
		t.Fatalf("missing lookup must report created issue and partial failure: %v", payload)
	}
	mu.Lock()
	got := calls
	mu.Unlock()
	if got != 1 {
		t.Fatalf("requests=%d want=1", got)
	}
}

func TestCreateBoardCreationFailureNeverRetries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		state  string
	}{
		{"rejected", http.StatusBadRequest, "not_applied"},
		{"unknown", http.StatusInternalServerError, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var calls []string
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				calls = append(calls, r.Method+" "+r.URL.Path)
				mu.Unlock()
				w.WriteHeader(tc.status)
			}))
			defer api.Close()
			ctx := context.Background()
			st, ct := mcp.NewInMemoryTransports()
			ss, err := New(Options{AllowWrites: true, Writer: jira.NewClient(api.URL, "user", "secret")}).Connect(ctx, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ss.Close()
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer cs.Close()
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_create", Arguments: map[string]any{"project": "ABC", "issueType": "Task", "summary": "Example", "boardId": 7}})
			if err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(res.StructuredContent)
			var payload map[string]any
			if err := json.Unmarshal(b, &payload); err != nil {
				t.Fatal(err)
			}
			if !res.IsError || payload["mutationState"] != tc.state || payload["issueKey"] != nil {
				t.Fatalf("unexpected creation outcome: %v", payload)
			}
			if tc.state == "unknown" && !strings.Contains(strings.ToLower(fmt.Sprint(payload["hint"])), "verif") {
				t.Fatalf("unknown creation must require verification: %v", payload)
			}
			mu.Lock()
			got := strings.Join(calls, ",")
			mu.Unlock()
			if got != "POST /rest/api/3/issue" {
				t.Fatalf("requests=%s", got)
			}
		})
	}
}

func TestCreateBoardPublicInterface(t *testing.T) {
	cases := []struct {
		name         string
		board        any
		parent       bool
		mode         string
		wantCode     string
		wantRequests string
	}{
		{"active", 7, false, "active", "", "POST issue,GET active,POST sprint"},
		{"fallback active", 7, false, "fallback", "", "POST issue,GET active,GET all,POST sprint"},
		{"lookup failure", 7, false, "lookup", "partial_failure", "POST issue,GET active"},
		{"no active", 7, false, "none", "partial_failure", "POST issue,GET active,GET all"},
		{"add rejected", 7, false, "reject", "partial_failure", "POST issue,GET active,POST sprint"},
		{"add unknown", 7, false, "unknown", "partial_failure", "POST issue,GET active,POST sprint"},
		{"zero", 0, false, "", "validation_error", ""},
		{"negative", -1, false, "", "validation_error", ""},
		{"fraction", 1.5, false, "", "validation_error", ""},
		{"string", "7", false, "", "validation_error", ""},
		{"null", nil, false, "", "validation_error", ""},
		{"overflow", 2147483648, false, "", "validation_error", ""},
		{"parent board", 7, true, "", "validation_error", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var calls []string
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				switch {
				case r.Method == "POST" && r.URL.Path == "/rest/api/3/issue":
					calls = append(calls, "POST issue")
					fmt.Fprint(w, `{"key":"ABC-1"}`)
				case r.Method == "GET" && r.URL.Path == "/rest/agile/1.0/board/7/sprint":
					if r.URL.Query().Get("state") == "active" {
						calls = append(calls, "GET active")
					} else {
						calls = append(calls, "GET all")
					}
					if tc.mode == "lookup" {
						w.WriteHeader(400)
						return
					}
					if tc.mode == "none" || (tc.mode == "fallback" && r.URL.Query().Get("state") == "active") {
						fmt.Fprint(w, `{"values":[]}`)
					} else {
						fmt.Fprint(w, `{"values":[{"id":42,"state":"active"}]}`)
					}
				case r.Method == "POST" && r.URL.Path == "/rest/agile/1.0/sprint/42/issue":
					calls = append(calls, "POST sprint")
					var body struct {
						Issues []string `json:"issues"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Issues) != 1 || body.Issues[0] != "ABC-1" {
						t.Errorf("invalid sprint body: %+v %v", body, err)
					}
					if tc.mode == "reject" {
						w.WriteHeader(400)
					}
					if tc.mode == "unknown" {
						w.WriteHeader(500)
					}
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
					w.WriteHeader(404)
				}
			}))
			defer api.Close()
			ctx := context.Background()
			st, ct := mcp.NewInMemoryTransports()
			ss, err := New(Options{AllowWrites: true, Writer: jira.NewClient(api.URL, "user", "secret")}).Connect(ctx, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ss.Close()
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer cs.Close()
			args := map[string]any{"project": "ABC", "issueType": "Task", "summary": "Example", "boardId": tc.board}
			if tc.parent {
				args["parent"] = "ABC-2"
				args["issueType"] = "Sub-task"
			}
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_create", Arguments: args})
			if err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(res.StructuredContent)
			var payload map[string]any
			if err := json.Unmarshal(b, &payload); err != nil {
				t.Fatal(err)
			}
			if tc.wantCode == "" {
				if res.IsError || payload["mutationState"] != "applied" || payload["issueKey"] != "ABC-1" {
					t.Fatalf("result=%+v payload=%v", res, payload)
				}
			} else if !res.IsError || payload["code"] != tc.wantCode || (tc.wantCode == "partial_failure" && (payload["issueKey"] != "ABC-1" || payload["mutationState"] != "applied")) {
				t.Fatalf("result=%+v payload=%v", res, payload)
			}
			if tc.wantCode == "validation_error" && (payload["mutationState"] != "not_applied" || payload["issueKey"] != nil) {
				t.Fatalf("validation must not mutate or return an issue key: %v", payload)
			}
			if tc.mode == "unknown" && !strings.Contains(strings.ToLower(fmt.Sprint(payload["hint"])), "verif") {
				t.Fatalf("unknown placement must require verification: %v", payload)
			}
			mu.Lock()
			got := strings.Join(calls, ",")
			mu.Unlock()
			if got != tc.wantRequests {
				t.Fatalf("requests=%s want=%s", got, tc.wantRequests)
			}
		})
	}
}
