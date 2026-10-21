package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func callWriteTool(t *testing.T, cs *mcp.ClientSession, name string, args any) (bool, map[string]any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	return res.IsError, got
}

func TestSprintMoveAndBacklogSendOneRequestToTheExpectedPath(t *testing.T) {
	for _, tc := range []struct {
		tool, action, path string
		args               map[string]any
	}{
		{"gojira_sprint_move", "sprint_move", "/rest/agile/1.0/sprint/42/issue", map[string]any{"key": "ABC-1", "sprintId": 42}},
		{"gojira_backlog", "backlog", "/rest/agile/1.0/backlog/issue", map[string]any{"key": "ABC-1"}},
		{"gojira_backlog", "backlog", "/rest/agile/1.0/backlog/issue", map[string]any{"key": " abc-1 "}},
	} {
		t.Run(tc.tool+" "+tc.args["key"].(string), func(t *testing.T) {
			var calls atomic.Int32
			cs := paritySession(t, true, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				raw, _ := io.ReadAll(r.Body)
				if r.Method != http.MethodPost || r.URL.Path != tc.path || strings.TrimSpace(string(raw)) != `{"issues":["ABC-1"]}` {
					t.Errorf("unexpected request %s %s %s", r.Method, r.URL.Path, raw)
				}
				w.WriteHeader(http.StatusNoContent)
			})
			isError, got := callWriteTool(t, cs, tc.tool, tc.args)
			if isError || got["action"] != tc.action || got["issueKey"] != "ABC-1" || got["mutationState"] != "applied" || got["schemaVersion"] != "v1" || got["code"] != nil || calls.Load() != 1 {
				t.Fatalf("isError=%v result=%v calls=%d", isError, got, calls.Load())
			}
		})
	}
}

func TestSprintMoveRejectsAnythingButAPositiveIntegerSprintId(t *testing.T) {
	for _, tc := range []struct {
		name, args, message string
	}{
		{"zero", `{"key":"ABC-1","sprintId":0}`, "sprintId must be a positive integer"},
		{"negative", `{"key":"ABC-1","sprintId":-1}`, "sprintId must be a positive integer"},
		{"string", `{"key":"ABC-1","sprintId":"5"}`, "sprintId must be a positive integer"},
		{"fraction", `{"key":"ABC-1","sprintId":1.5}`, "sprintId must be a positive integer"},
		{"exponent", `{"key":"ABC-1","sprintId":1e2}`, "sprintId must be a positive integer"},
		{"boolean", `{"key":"ABC-1","sprintId":true}`, "sprintId must be a positive integer"},
		{"beyond the Jira integer range", `{"key":"ABC-1","sprintId":2147483648}`, "sprintId must be a positive integer"},
		{"null", `{"key":"ABC-1","sprintId":null}`, "arguments must not be null"},
		{"missing", `{"key":"ABC-1"}`, "required argument missing"},
		{"unexpected board argument", `{"key":"ABC-1","sprintId":5,"boardId":1}`, "unexpected argument"},
		{"invalid key", `{"key":"not a key","sprintId":5}`, "invalid issue key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			cs := paritySession(t, true, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			})
			isError, got := callWriteTool(t, cs, "gojira_sprint_move", json.RawMessage(tc.args))
			if !isError || got["code"] != "validation_error" || got["message"] != tc.message || got["mutationState"] != "not_applied" || got["hint"] != "Correct arguments before retrying." || calls.Load() != 0 {
				t.Fatalf("isError=%v result=%v calls=%d", isError, got, calls.Load())
			}
		})
	}
}

func TestBacklogRejectsInvalidInputWithoutAnyRequest(t *testing.T) {
	for _, tc := range []struct {
		name, args, message string
	}{
		{"invalid key", `{"key":"not a key"}`, "invalid issue key"},
		{"missing key", `{}`, "required argument missing"},
		{"blank key", `{"key":" "}`, "required arguments must not be empty"},
		{"unexpected sprint argument", `{"key":"ABC-1","sprintId":5}`, "unexpected argument"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			cs := paritySession(t, true, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			})
			isError, got := callWriteTool(t, cs, "gojira_backlog", json.RawMessage(tc.args))
			if !isError || got["code"] != "validation_error" || got["message"] != tc.message || got["mutationState"] != "not_applied" || calls.Load() != 0 {
				t.Fatalf("isError=%v result=%v calls=%d", isError, got, calls.Load())
			}
		})
	}
}

func TestSprintMoveAndBacklogReportTheMutationStateAndNeverRetry(t *testing.T) {
	for _, tc := range []struct {
		name, tool string
		args       map[string]any
		status     int
		state      string
		hintHas    string
	}{
		{"sprint move server failure is unknown", "gojira_sprint_move", map[string]any{"key": "ABC-1", "sprintId": 7}, http.StatusInternalServerError, "unknown", "Verify the issue in Jira before retrying"},
		{"backlog server failure is unknown", "gojira_backlog", map[string]any{"key": "ABC-1"}, http.StatusInternalServerError, "unknown", "Verify the issue in Jira before retrying"},
		{"sprint move rejected is not applied", "gojira_sprint_move", map[string]any{"key": "ABC-1", "sprintId": 7}, http.StatusForbidden, "not_applied", "Check Jira access before retrying."},
		{"backlog rejected is not applied", "gojira_backlog", map[string]any{"key": "ABC-1"}, http.StatusBadRequest, "not_applied", "Check Jira access before retrying."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			cs := paritySession(t, true, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
			})
			isError, got := callWriteTool(t, cs, tc.tool, tc.args)
			hint, _ := got["hint"].(string)
			if !isError || got["mutationState"] != tc.state || got["code"] != "mutation_failed" || !strings.Contains(hint, tc.hintHas) || calls.Load() != 1 {
				t.Fatalf("isError=%v result=%v calls=%d", isError, got, calls.Load())
			}
		})
	}
}
