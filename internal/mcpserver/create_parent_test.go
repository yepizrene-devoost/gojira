package mcpserver

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCreateParentPublicInterface(t *testing.T) {
	cases := []struct {
		name      string
		parent    any
		issueType string
		valid     bool
	}{
		{"valid", "ABC-12", "Sub-task", true},
		{"empty", "", "Sub-task", false},
		{"whitespace", " ABC-12", "Sub-task", false},
		{"lowercase", "abc-12", "Sub-task", false},
		{"zero", "ABC-0", "Sub-task", false},
		{"injection", "ABC-12/extra", "Sub-task", false},
		{"wrong type", "ABC-12", "Task", false},
		{"null", nil, "Sub-task", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, _, log := writeTestAdapter(t, 200, "", time.Now)
			ctx := context.Background()
			st, ct := mcp.NewInMemoryTransports()
			ss, err := New(Options{AllowWrites: true, Writer: a.Writer}).Connect(ctx, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = ss.Close() }()
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cs.Close() }()
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_create", Arguments: map[string]any{"project": "ABC", "issueType": tc.issueType, "summary": "Example", "parent": tc.parent}})
			if err != nil {
				t.Fatal(err)
			}
			if tc.valid {
				if res.IsError {
					t.Fatalf("result: %+v", res)
				}
				want := []observedWriteRequest{{"POST", "/rest/api/3/issue", decodedWriteJSON(t, `{"fields":{"project":{"key":"ABC"},"issuetype":{"name":"Sub-task"},"summary":"Example","parent":{"key":"ABC-12"}}}`)}}
				if got := log.snapshot(); !reflect.DeepEqual(got, want) {
					t.Fatalf("requests=%v want %v", got, want)
				}
			} else {
				if !res.IsError {
					b, _ := json.Marshal(res)
					t.Fatalf("expected error: %s", b)
				}
				if got := log.snapshot(); len(got) != 0 {
					t.Fatalf("unexpected requests: %v", got)
				}
			}
		})
	}
}
