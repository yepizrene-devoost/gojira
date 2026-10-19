package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestLifecycleCLI(t *testing.T) {
	for _, tc := range []struct {
		name, command, transitions, code, id string
		status                               int
	}{
		{"close", "close", `[{"id":"1","name":"Finish","to":{"name":"Complete","statusCategory":{"key":"done"}}}]`, "", "1", 204},
		{"reopen new", "reopen", `[{"id":"2","name":"Start","to":{"statusCategory":{"key":"new"}}}]`, "", "2", 204},
		{"reopen ongoing", "reopen", `[{"id":"3","name":"Resume","to":{"statusCategory":{"key":"indeterminate"}}}]`, "", "3", 204},
		{"missing destination", "close", `[{"id":"1","name":"Done"}]`, "validation_error", "", 204},
		{"none", "close", `[]`, "validation_error", "", 204},
		{"ambiguous", "close", `[{"id":"1","name":"A","to":{"name":"Done","statusCategory":{"key":"done"}}},{"id":"2","name":"B","to":{"statusCategory":{"key":"done"}}}]`, "validation_error", "", 204},
		{"denied", "close", `[]`, "permission_denied", "", 403},
		{"write denied", "close", `[{"id":"1","name":"Finish","to":{"statusCategory":{"key":"done"}}}]`, "permission_denied", "1", 403},
		{"write server failure", "close", `[{"id":"1","name":"Finish","to":{"statusCategory":{"key":"done"}}}]`, "mutation_failed", "1", 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					writes++
					b, _ := io.ReadAll(r.Body)
					if r.URL.Path != "/rest/api/3/issue/A-1/transitions" || string(b) != `{"transition":{"id":"`+tc.id+`"}}` {
						t.Errorf("unexpected write %s %s", r.URL, b)
					}
					w.WriteHeader(tc.status)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/transitions") {
					if tc.status == 403 && tc.id == "" {
						w.WriteHeader(403)
						return
					}
					_, _ = fmt.Fprintf(w, `{"transitions":%s}`, tc.transitions)
					return
				}
				_, _ = fmt.Fprint(w, `{"key":"A-1","fields":{}}`)
			}))
			defer srv.Close()
			exit, out, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), tc.command, "A-1", "--json")
			if tc.code != "" {
				if exit != 1 || out != "" || !strings.Contains(stderr, `"code": "`+tc.code+`"`) {
					t.Fatalf("%d %s %s", exit, out, stderr)
				}
				if tc.status == 500 && !strings.Contains(stderr, `"mutationState": "unknown"`) {
					t.Fatal(stderr)
				}
				if tc.name == "ambiguous" && (!strings.Contains(stderr, "1: A") || !strings.Contains(stderr, "2: B")) {
					t.Fatal(stderr)
				}
			} else if exit != 0 || !strings.Contains(out, `"key": "A-1"`) {
				t.Fatalf("%d %s %s", exit, out, stderr)
			}
			expected := 0
			if tc.id != "" {
				expected = 1
			}
			if writes != expected {
				t.Fatalf("writes=%d", writes)
			}
		})
	}
}
