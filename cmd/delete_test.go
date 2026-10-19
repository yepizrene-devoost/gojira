package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestDeleteCLI(t *testing.T) {
	for _, tc := range []struct {
		name, key, confirm, code, state string
		status                          int
	}{
		{"success", "A-1", "A-1", "", "", 204},
		{"missing confirmation", "A-1", "", "validation_error", "not_applied", 204},
		{"mismatch", "A-1", "a-1", "validation_error", "not_applied", 204},
		{"invalid key", "bad", "bad", "validation_error", "not_applied", 204},
		{"denied", "A-1", "A-1", "permission_denied", "not_applied", 403},
		{"server failure", "A-1", "A-1", "mutation_failed", "unknown", 500},
		{"transport drop", "A-1", "A-1", "mutation_failed", "unknown", -1},
		{"subtasks", "A-1", "A-1", "mutation_failed", "not_applied", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "DELETE" || r.URL.Path != "/rest/api/3/issue/A-1" || r.URL.RawQuery != "" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
				}
				if tc.status == -1 {
					c, _, _ := w.(http.Hijacker).Hijack()
					_ = c.Close()
					return
				}
				w.WriteHeader(tc.status)
				if tc.status == 400 {
					_, _ = fmt.Fprint(w, `parent has subtasks`)
				}
			}))
			defer srv.Close()
			args := []string{"delete", tc.key, "--json"}
			if tc.confirm != "" {
				args = append(args, "--confirm", tc.confirm)
			}
			exit, out, stderr := runTestCLI(t, jira.NewClient(srv.URL, "email", "token"), args...)
			expected := 1
			if tc.code == "validation_error" {
				expected = 0
			}
			if calls != expected {
				t.Fatalf("calls=%d", calls)
			}
			if tc.code == "" {
				if exit != 0 || !strings.Contains(out, `"deleted": true`) || !strings.Contains(out, `"schemaVersion": "v1"`) {
					t.Fatalf("%d %s %s", exit, out, stderr)
				}
			} else if exit != 1 || out != "" || !strings.Contains(stderr, `"code": "`+tc.code+`"`) || !strings.Contains(stderr, `"mutationState": "`+tc.state+`"`) {
				t.Fatalf("%d %s %s", exit, out, stderr)
			}
			if tc.status == 403 && (!strings.Contains(stderr, `"httpStatus": 403`) || !strings.Contains(stderr, "not applied")) {
				t.Fatal(stderr)
			}
			if tc.state == "unknown" && !strings.Contains(stderr, "verify before retrying") {
				t.Fatal(stderr)
			}
		})
	}
}
