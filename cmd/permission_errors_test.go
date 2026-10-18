package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestPermissionErrors(t *testing.T) {
	commands := [][]string{
		{"get", "A-1"}, {"search", "project=A"}, {"boards"}, {"projects"},
		{"export", "--board", "1", "--sprint", "1"}, {"move", "A-1"}, {"move", "A-1", "--to", "Done"},
		{"update", "A-1", "--summary", "new"}, {"comment", "A-1", "hello"}, {"assign", "A-1", "a@example.test"},
		{"log", "A-1", "--time", "1h"}, {"backlog", "A-1"}, {"create", "--project", "A", "--summary", "new"},
	}
	for _, status := range []int{401, 403, 404, 500} {
		for _, args := range commands {
			t.Run(fmt.Sprintf("%d/%s", status, strings.Join(args, "_")), func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
				defer srv.Close()
				if args[0] != "export" {
					args = append(append([]string{}, args...), "--json")
				}
				exit, out, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), args...)
				var envelope struct {
					Code          string
					HTTPStatus    int `json:"httpStatus"`
					Hint          string
					MutationState string
				}
				if exit != 1 || out != "" || json.Unmarshal([]byte(stderr), &envelope) != nil {
					t.Fatalf("exit=%d out=%s err=%s", exit, out, stderr)
				}
				if status == 500 {
					if envelope.Hint != "" || envelope.HTTPStatus != 0 {
						t.Fatalf("unclassified: %s", stderr)
					}
					return
				}
				want := map[int]string{401: "authentication_failed", 403: "permission_denied", 404: "not_found_or_no_permission"}[status]
				if envelope.Code != want || envelope.HTTPStatus != status || envelope.Hint == "" || envelope.MutationState != "not_applied" {
					t.Fatalf("envelope=%s", stderr)
				}
			})
		}
	}
}

func TestSprintPermissionWarning(t *testing.T) {
	for _, jsonMode := range []bool{true, false} {
		t.Run(fmt.Sprint(jsonMode), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 1 {
					_, _ = fmt.Fprint(w, `{"key":"A-1","fields":{}}`)
				} else {
					w.WriteHeader(403)
				}
			}))
			defer srv.Close()
			args := []string{"get", "A-1"}
			if jsonMode {
				args = append(args, "--json")
			}
			exit, out, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), args...)
			if exit != 0 {
				t.Fatalf("exit=%d %s", exit, stderr)
			}
			if jsonMode {
				if !json.Valid([]byte(out)) || !strings.Contains(out, `"warnings"`) || !strings.Contains(out, "permission_denied") || stderr != "" {
					t.Fatalf("out=%s err=%s", out, stderr)
				}
			} else if !strings.Contains(stderr, "sprint_unavailable") {
				t.Fatalf("warning=%s", stderr)
			}
		})
	}
}
