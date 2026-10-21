package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

type observedWriteRequest struct {
	Method, Path string
	Body         any
}
type writeRequestLog struct {
	mu       sync.Mutex
	requests []observedWriteRequest
}

func (log *writeRequestLog) snapshot() []observedWriteRequest {
	log.mu.Lock()
	defer log.mu.Unlock()
	return append([]observedWriteRequest{}, log.requests...)
}
func writeTestAdapter(t *testing.T, status int, transitions string, now func() time.Time) (adapter, *writeConfirmations, *writeRequestLog) {
	t.Helper()
	log := &writeRequestLog{}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body any
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("invalid request JSON: %s", raw)
			}
		}
		log.mu.Lock()
		log.requests = append(log.requests, observedWriteRequest{r.Method, r.URL.Path, body})
		log.mu.Unlock()
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(transitions))
			return
		}
		w.WriteHeader(status)
		if status == 200 {
			_, _ = w.Write([]byte(`{"key":"ABC-1"}`))
		} else {
			_, _ = w.Write([]byte(`SECRET_MARKER https://private.invalid/private`))
		}
	}))
	t.Cleanup(api.Close)
	return adapter{Options: Options{Writer: jira.NewClient(api.URL, "user", "secret")}}, newWriteConfirmations(now), log
}
func callWriteTest(t *testing.T, a adapter, store *writeConfirmations, action string, args map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range writeTools() {
		if entry.action == action {
			res, err := a.writeCall(entry, store)(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: raw}})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(res)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"SECRET_MARKER", "https://", "private.invalid", "127.0.0.1"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("leaked error detail: %s", encoded)
				}
			}
			payload, _ := json.Marshal(res.StructuredContent)
			var value map[string]any
			if err = json.Unmarshal(payload, &value); err != nil {
				t.Fatal(err)
			}
			return value
		}
	}
	t.Fatalf("missing action %s", action)
	return nil
}
func decodedWriteJSON(t *testing.T, raw string) any {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

const expectedWriteADF = `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Example"}]}]}`

func TestWriteMutationRequests(t *testing.T) {
	cases := []struct {
		action, method, path, body string
		args                       map[string]any
	}{
		{"create", "POST", "/rest/api/3/issue", `{"fields":{"project":{"key":"ABC"},"issuetype":{"name":"Task"},"summary":"Example"}}`, map[string]any{"project": "ABC", "issueType": "Task", "summary": "Example"}},
		{"create", "POST", "/rest/api/3/issue", `{"fields":{"project":{"key":"ABC"},"issuetype":{"name":"Task"},"summary":"Example","description":` + expectedWriteADF + `}}`, map[string]any{"project": "ABC", "issueType": "Task", "summary": "Example", "description": "Example"}},
		{"comment", "POST", "/rest/api/3/issue/ABC-1/comment", `{"body":` + expectedWriteADF + `}`, map[string]any{"key": "ABC-1", "body": "Example"}},
		{"log_work", "POST", "/rest/api/3/issue/ABC-1/worklog", `{"timeSpent":"2h","comment":"Example"}`, map[string]any{"key": "ABC-1", "timeSpent": "2h", "comment": "Example"}},
		{"assign", "PUT", "/rest/api/3/issue/ABC-1/assignee", `{"accountId":"account"}`, map[string]any{"key": "ABC-1", "accountId": "account"}},
		{"move", "POST", "/rest/api/3/issue/ABC-1/transitions", `{"transition":{"id":"chosen"}}`, map[string]any{"key": "ABC-1", "status": "Done"}},
		{"update", "PUT", "/rest/api/3/issue/ABC-1", `{"fields":{"summary":"Example","priority":{"name":"High"}}}`, map[string]any{"key": "ABC-1", "fields": map[string]any{"summary": "Example", "priority": "High"}}},
		{"delete", "DELETE", "/rest/api/3/issue/ABC-1", "", map[string]any{"key": "ABC-1"}},
	}
	for i, tc := range cases {
		for _, status := range []int{200, 400, 500} {
			t.Run(fmt.Sprintf("%s/%d/%d", tc.action, i, status), func(t *testing.T) {
				a, store, log := writeTestAdapter(t, status, `{"transitions":[{"id":"other","name":"Open"},{"id":"chosen","name":"Done"}]}`, time.Now)
				args := map[string]any{}
				for key, value := range tc.args {
					args[key] = value
				}
				if tc.action == "delete" {
					preview := callWriteTest(t, a, store, "delete", args)
					args["confirmationToken"] = preview["data"].(map[string]any)["confirmationToken"]
					if len(log.snapshot()) != 0 {
						t.Fatal("preview requested Jira")
					}
				}
				res := callWriteTest(t, a, store, tc.action, args)
				wantState := "applied"
				if status == 400 {
					wantState = "not_applied"
				}
				if status == 500 {
					wantState = "unknown"
					if !strings.Contains(res["hint"].(string), "before retrying") {
						t.Fatal("unknown lacks verify hint")
					}
				}
				if res["mutationState"] != wantState {
					t.Fatalf("result=%v", res)
				}
				var body any
				if tc.body != "" {
					body = decodedWriteJSON(t, tc.body)
				}
				want := []observedWriteRequest{{tc.method, tc.path, body}}
				if tc.action == "move" {
					want = append([]observedWriteRequest{{"GET", "/rest/api/3/issue/ABC-1/transitions", nil}}, want...)
				}
				if got := log.snapshot(); !reflect.DeepEqual(got, want) {
					t.Fatalf("requests=%+v want %+v", got, want)
				}
			})
		}
	}
}
func TestWriteMoveAmbiguityRequests(t *testing.T) {
	for _, transitions := range []string{`{"transitions":[]}`, `{"transitions":[{"id":"1","name":"Done"},{"id":"2","name":"Done"}]}`} {
		t.Run(transitions, func(t *testing.T) {
			a, store, log := writeTestAdapter(t, 200, transitions, time.Now)
			res := callWriteTest(t, a, store, "move", map[string]any{"key": "ABC-1", "status": "Done"})
			if res["code"] != "validation_error" || res["mutationState"] != "not_applied" {
				t.Fatalf("%v", res)
			}
			if got := log.snapshot(); !reflect.DeepEqual(got, []observedWriteRequest{{"GET", "/rest/api/3/issue/ABC-1/transitions", nil}}) {
				t.Fatalf("requests=%v", got)
			}
		})
	}
}
func TestWriteDescriptionRejectedWithoutRequests(t *testing.T) {
	// Plain text replacement is supported; malformed descriptions still cannot
	// reach Jira, including when a valid description accompanies an unknown field.
	for _, fields := range []map[string]any{
		{"description": 42},
		{"description": nil},
		{"description": "replace", "unknown": "value"},
		{},
	} {
		a, store, log := writeTestAdapter(t, 200, "", time.Now)
		res := callWriteTest(t, a, store, "update", map[string]any{"key": "ABC-1", "fields": fields})
		if res["code"] != "validation_error" || res["mutationState"] != "not_applied" || len(log.snapshot()) != 0 {
			t.Fatalf("result=%v requests=%v", res, log.snapshot())
		}
	}
}
