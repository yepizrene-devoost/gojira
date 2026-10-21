package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// TestCloseReopenSelectionBaseline pins the exact selection outcome of `close` and
// `reopen` so moving the rule into a shared package cannot change it.
func TestCloseReopenSelectionBaseline(t *testing.T) {
	for _, tc := range []struct {
		name, command, transitions string
		wantID, wantMessage        string
	}{
		{"close picks the only done transition", "close", `[{"id":"7","name":"Start","to":{"name":"Doing","statusCategory":{"key":"indeterminate"}}},{"id":"9","name":"Finish","to":{"name":"Done","statusCategory":{"key":"done"}}}]`, "9", ""},
		{"reopen picks the only new transition", "reopen", `[{"id":"4","name":"Finish","to":{"name":"Done","statusCategory":{"key":"done"}}},{"id":"5","name":"Reopen","to":{"name":"To Do","statusCategory":{"key":"new"}}}]`, "5", ""},
		{"close without candidates", "close", `[]`, "", "close requires exactly one available transition; candidates: none"},
		{"close ignores transitions without a destination category", "close", `[{"id":"1","name":"Done"},{"id":"2","name":"Other","to":{"name":"Other"}}]`, "", "close requires exactly one available transition; candidates: none"},
		{"close ambiguous", "close", `[{"id":"1","name":"A","to":{"name":"Done","statusCategory":{"key":"done"}}},{"id":"2","name":"B","to":{"statusCategory":{"key":"done"}}}]`, "", "close requires exactly one available transition; candidates: 1: A -> Done (done); 2: B ->  (done)"},
		{"reopen ambiguous", "reopen", `[{"id":"3","name":"X","to":{"name":"Open","statusCategory":{"key":"new"}}},{"id":"4","name":"Y","to":{"name":"Doing","statusCategory":{"key":"indeterminate"}}}]`, "", "reopen requires exactly one available transition; candidates: 3: X -> Open (new); 4: Y -> Doing (indeterminate)"},
		{"reopen with only done transitions", "reopen", `[{"id":"4","name":"Finish","to":{"name":"Done","statusCategory":{"key":"done"}}}]`, "", "reopen requires exactly one available transition; candidates: none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			var body string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					writes++
					raw, _ := io.ReadAll(r.Body)
					body = string(raw)
					w.WriteHeader(http.StatusNoContent)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/transitions") {
					_, _ = fmt.Fprintf(w, `{"transitions":%s}`, tc.transitions)
					return
				}
				_, _ = fmt.Fprint(w, `{"key":"A-1","fields":{}}`)
			}))
			defer srv.Close()
			exit, out, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), tc.command, "A-1", "--json")
			if tc.wantMessage == "" {
				if exit != 0 || writes != 1 || body != `{"transition":{"id":"`+tc.wantID+`"}}` || !strings.Contains(out, `"key": "A-1"`) {
					t.Fatalf("exit=%d writes=%d body=%s out=%s stderr=%s", exit, writes, body, out, stderr)
				}
				return
			}
			var envelope map[string]any
			if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
				t.Fatalf("stderr is not one JSON document: %v: %s", err, stderr)
			}
			if exit != 1 || out != "" || writes != 0 || envelope["code"] != "validation_error" || envelope["message"] != tc.wantMessage || envelope["mutationState"] != "not_applied" {
				t.Fatalf("exit=%d writes=%d out=%q envelope=%v", exit, writes, out, envelope)
			}
		})
	}
}
