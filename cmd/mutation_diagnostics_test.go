package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

func TestMutationDiagnostics(t *testing.T) {
	email, token := "diagnostic@example.test", "TOKEN_MARKER"
	encoded := base64.StdEncoding.EncodeToString([]byte(email + ":" + token + ":"))
	cases := []struct {
		name, body string
		status     int
		field      string
		absent     bool
	}{
		{"mandatory_field", `{"errors":{"summary":"Summary is required","issuetype":"Invalid issue type"},"errorMessages":["Cannot create issue"]}`, 400, "errors", false},
		{"messages", `{"errorMessages":["Cannot create issue"]}`, 400, "errorMessages", false},
		{"redact_keys_and_values", fmt.Sprintf(`{"errors":{%q:%q},"errorMessages":[%q]}`, token, email+" "+token, encoded), 400, "errors", false},
		{"beyond_old_truncation", `{"ignored":"` + strings.Repeat("x", 300) + `","errors":{"summary":"Required"}}`, 400, "errors", false},
		{"malformed", `SECRET_MARKER {`, 400, "errors", true},
		{"oversized", `{"errors":{"summary":"` + strings.Repeat("x", 70000) + `"}}`, 400, "errors", true},
		{"bounded_message", `{"errors":{"summary":"` + strings.Repeat("x", 2000) + `"}}`, 400, "errors", false},
		{"unauthorized", `{}`, 401, "errors", true},
		{"forbidden", `{}`, 403, "errors", true},
		{"missing", `{}`, 404, "errors", true},
		{"unknown", `{}`, 500, "errors", true},
		{"transport", ``, 0, "errors", true},
		{"unallowlisted", `{"body":"SECRET_MARKER","errors":{"summary":"Required"},"stack":"SECRET_MARKER"}`, 400, "errors", false},
	}
	for _, tc := range cases {
		for _, adapter := range []string{"cli", "mcp"} {
			for _, action := range []string{"create", "update"} {
				t.Run(tc.name+"/"+adapter+"/"+action, func(t *testing.T) {
					calls := 0
					api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls++
						if tc.status == 0 {
							conn, _, err := w.(http.Hijacker).Hijack()
							if err == nil {
								_ = conn.Close()
							}
							return
						}
						w.WriteHeader(tc.status)
						_, _ = fmt.Fprint(w, tc.body)
					}))
					defer api.Close()
					client := jira.NewClient(api.URL, email, token)
					var raw []byte
					if adapter == "cli" {
						args := []string{"create", "--project", "ARA", "--type", "Bug", "--summary", "Example", "--json"}
						if action == "update" {
							args = []string{"update", "ARA-1", "--summary", "Example", "--json"}
						}
						exit, out, stderr := runTestCLI(t, client, args...)
						if exit != 1 || out != "" {
							t.Fatalf("exit=%d out=%s", exit, out)
						}
						raw = []byte(stderr)
					} else {
						ctx := context.Background()
						st, ct := mcp.NewInMemoryTransports()
						ss, err := newMCPServer(client, "example.invalid", true).Connect(ctx, st, nil)
						if err != nil {
							t.Fatal(err)
						}
						defer func() { _ = ss.Close() }()
						cs, err := mcp.NewClient(&mcp.Implementation{Name: "diagnostics", Version: "1"}, nil).Connect(ctx, ct, nil)
						if err != nil {
							t.Fatal(err)
						}
						defer func() { _ = cs.Close() }()
						args := map[string]any{"project": "ARA", "issueType": "Bug", "summary": "Example"}
						if action == "update" {
							args = map[string]any{"key": "ARA-1", "fields": map[string]any{"summary": "Example"}}
						}
						res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_" + action, Arguments: args})
						if err != nil {
							t.Fatal(err)
						}
						raw, _ = json.Marshal(res.StructuredContent)
					}
					var envelope map[string]any
					if err := json.Unmarshal(raw, &envelope); err != nil {
						t.Fatalf("invalid envelope: %s", raw)
					}
					// Check each diagnostic rule independently in its own scenario.
					if tc.name == "redact_keys_and_values" {
						for _, secret := range []string{email, token, encoded} {
							if strings.Contains(string(raw), secret) {
								t.Fatalf("credential leaked: %s", raw)
							}
						}
					}
					if tc.absent {
						if _, ok := envelope[tc.field]; ok {
							t.Fatalf("unexpected diagnostics: %s", raw)
						}
					} else if _, ok := envelope[tc.field]; !ok {
						t.Errorf("missing %s: %s", tc.field, raw)
					}
					if len(raw) > 10000 {
						t.Errorf("unbounded diagnostics: %d", len(raw))
					}
					if tc.status != 0 && envelope["httpStatus"] != float64(tc.status) {
						t.Errorf("missing HTTP status: %s", raw)
					}
					state := "not_applied"
					if tc.status == 500 || tc.status == 0 {
						state = "unknown"
					}
					if envelope["mutationState"] != state || calls != 1 {
						t.Errorf("state=%v calls=%d", envelope["mutationState"], calls)
					}
					wantCode := map[int]string{401: "authentication_failed", 403: "permission_denied", 404: "not_found_or_no_permission"}[tc.status]
					if wantCode == "" {
						wantCode = "mutation_failed"
					}
					if envelope["code"] != wantCode || envelope["schemaVersion"] != "v1" {
						t.Errorf("legacy envelope changed: %s", raw)
					}
					if tc.name == "mandatory_field" {
						fields, _ := envelope["errors"].(map[string]any)
						if fields["summary"] != "Summary is required" || fields["issuetype"] != "Invalid issue type" {
							t.Errorf("lost Jira fields: %s", raw)
						}
					}
					if tc.name == "bounded_message" {
						fields, _ := envelope["errors"].(map[string]any)
						message, _ := fields["summary"].(string)
						if len(message) != 256 {
							t.Errorf("message length=%d", len(message))
						}
					}
					if strings.Contains(string(raw), "SECRET_MARKER") {
						t.Fatal("raw body leaked")
					}
				})
			}
		}
	}
}
