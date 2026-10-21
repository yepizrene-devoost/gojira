package mcpserver

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWriteEnabledSurface(t *testing.T) {
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := New(Options{AllowWrites: true}).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ss.Close() }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "surface", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()
	list, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"gojira_boards", "gojira_context", "gojira_get", "gojira_projects", "gojira_report_time", "gojira_search", "gojira_sprints", "gojira_users_search", "gojira_create", "gojira_comment", "gojira_log_work", "gojira_assign", "gojira_move", "gojira_update", "gojira_delete"}
	if len(list.Tools) != len(names) {
		t.Fatalf("tools=%d", len(list.Tools))
	}
	schemas := []string{
		`{"type":"object","properties":{"project":{"type":"string"},"issueType":{"type":"string"},"summary":{"type":"string"},"description":{"type":"string"}},"required":["project","issueType","summary"],"additionalProperties":false}`,
		`{"type":"object","properties":{"key":{"type":"string"},"body":{"type":"string"}},"required":["key","body"],"additionalProperties":false}`,
		`{"type":"object","properties":{"key":{"type":"string"},"timeSpent":{"type":"string"},"comment":{"type":"string"}},"required":["key","timeSpent"],"additionalProperties":false}`,
		`{"type":"object","properties":{"key":{"type":"string"},"accountId":{"type":"string"}},"required":["key","accountId"],"additionalProperties":false}`,
		`{"type":"object","properties":{"key":{"type":"string"},"status":{"type":"string"}},"required":["key","status"],"additionalProperties":false}`,
		`{"type":"object","properties":{"key":{"type":"string"},"fields":{"type":"object","properties":{"summary":{"type":"string"},"priority":{"type":"string"},"description":{"type":"string","description":"Plain text converted to ADF; empty string clears description. Replacing a description this way may discard rich ADF formatting that plain text cannot represent"},"dueDate":{"type":"string","description":"Real calendar date YYYY-MM-DD; empty string clears dueDate."},"labels":{"type":"array","items":{"type":"string"},"minItems":1},"components":{"type":"array","items":{"type":"string"},"description":"Component names; [] clears components."},"fixVersions":{"type":"array","items":{"type":"string"},"description":"Fix version names; [] clears fixVersions."},"storyPoints":{"type":["number","null"],"description":"Finite decimal number; null clears storyPoints. Requires storyPointsField."},"storyPointsField":{"type":"string"}},"required":[],"additionalProperties":false,"minProperties":1}},"required":["key","fields"],"additionalProperties":false}`,
		`{"type":"object","properties":{"key":{"type":"string"},"confirmationToken":{"type":"string"}},"required":["key"],"additionalProperties":false}`,
	}
	for i, tool := range list.Tools {
		if tool.Name != names[i] {
			t.Errorf("tool[%d]=%s want %s", i, tool.Name, names[i])
			continue
		}
		if i < 8 {
			continue
		}
		if tool.Name == "gojira_update" {
			for _, phrase := range []string{"plain text converted to ADF", "Replacing a description this way may discard rich ADF formatting that plain text cannot represent", "empty string clears description and dueDate", "[] clears components and fixVersions", "null clears storyPoints"} {
				if !strings.Contains(tool.Description, phrase) {
					t.Errorf("update description missing %q", phrase)
				}
			}
		}
		raw, _ := json.Marshal(tool.InputSchema)
		var got, want any
		_ = json.Unmarshal(raw, &got)
		_ = json.Unmarshal([]byte(schemas[i-8]), &want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s schema=%s want %s", tool.Name, raw, schemas[i-8])
		}
		destructive := i == 14
		expected := &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive, IdempotentHint: i == 11 || i == 13}
		if !reflect.DeepEqual(tool.Annotations, expected) {
			t.Errorf("%s annotations=%+v want %+v", tool.Name, tool.Annotations, expected)
		}
	}
}
