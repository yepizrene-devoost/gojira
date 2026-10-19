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

func TestSprintCLI(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		status int
		code   string
		state  string
	}{
		{"list", []string{"list", "--board", "2"}, 200, "", ""},
		{"filter", []string{"list", "--board", "2", "--state", "closed"}, 200, "", ""},
		{"list denied", []string{"list", "--board", "2"}, 403, "permission_denied", "not_applied"},
		{"move", []string{"move", "A-1", "--to", "3"}, 204, "", ""},
		{"move rejected", []string{"move", "A-1", "--to", "3"}, 500, "mutation_failed", "unknown"},
		{"move denied", []string{"move", "A-1", "--to", "3"}, 403, "permission_denied", "not_applied"},
		{"move drop", []string{"move", "A-1", "--to", "3"}, -1, "mutation_failed", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes, reads := 0, 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					writes++
					b, _ := io.ReadAll(r.Body)
					if r.URL.Path != "/rest/agile/1.0/sprint/3/issue" || string(b) != `{"issues":["A-1"]}` {
						t.Errorf("request %s %s", r.URL.Path, b)
					}
				} else {
					reads++
				}
				if tc.status == -1 {
					c, _, _ := w.(http.Hijacker).Hijack()
					_ = c.Close()
					return
				}
				if tc.args[0] == "list" {
					state := "active,future"
					if tc.name == "filter" {
						state = "closed"
					}
					if r.URL.Path != "/rest/agile/1.0/board/2/sprint" || r.URL.Query().Get("state") != state {
						t.Errorf("URL %s", r.URL)
					}
				}
				if r.Method == "GET" && tc.args[0] == "move" {
					_, _ = fmt.Fprint(w, `{"key":"A-1","fields":{}}`)
					return
				}
				w.WriteHeader(tc.status)
				if tc.status == 200 {
					_, _ = fmt.Fprint(w, `{"values":[]}`)
				}
			}))
			defer srv.Close()
			args := append([]string{"sprint"}, tc.args...)
			args = append(args, "--json")
			exit, out, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), args...)
			if tc.code != "" {
				var e struct {
					Code          string
					MutationState string
					HTTPStatus    int
					Hint          string
				}
				if exit != 1 || out != "" || json.Unmarshal([]byte(stderr), &e) != nil || e.Code != tc.code || e.MutationState != tc.state {
					t.Fatalf("exit=%d out=%s err=%s", exit, out, stderr)
				}
				if tc.status == 403 && (e.HTTPStatus != 403 || e.Hint == "") {
					t.Fatal(stderr)
				}
			} else if exit != 0 || !json.Valid([]byte(out)) || (tc.args[0] == "list" && strings.TrimSpace(out) != "[]") {
				t.Fatalf("exit=%d out=%s err=%s", exit, out, stderr)
			}
			if tc.args[0] == "move" && writes != 1 {
				t.Fatalf("writes=%d", writes)
			}
			if tc.args[0] == "list" && reads != 1 {
				t.Fatalf("reads=%d", reads)
			}
		})
	}
}

func TestSprintValidation(t *testing.T) {
	for _, args := range [][]string{{"list"}, {"list", "--board", "0"}, {"list", "--board", "bad"}, {"list", "--board", "1", "--state", "bad"}, {"move", "bad", "--to", "1"}, {"move", "A-1", "--to", "0"}, {"move", "A-1", "--to", "bad"}, {"move", "A-1"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
			defer srv.Close()
			argv := append([]string{"sprint"}, args...)
			argv = append(argv, "--json")
			exit, out, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), argv...)
			if exit != 1 || out != "" || !strings.Contains(stderr, `"validation_error"`) || calls != 0 {
				t.Fatalf("exit=%d calls=%d out=%s err=%s", exit, calls, out, stderr)
			}
		})
	}
}
