package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A nil Jira client must never become a non-nil Writer interface: the write
// handlers would then dereference it instead of reporting a configuration error.
func TestMCPWritesNilClientReportsConfigurationError(t *testing.T) {
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := newMCPServer(nil, "example.invalid", true).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ss.Close() }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "nil-writer-test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gojira_comment", Arguments: map[string]any{"key": "DEMO-1", "body": "Example"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected an error result")
	}
	text := ""
	for _, content := range result.Content {
		if value, ok := content.(*mcp.TextContent); ok {
			text += value.Text
		}
	}
	if !strings.Contains(text, "configuration_error") || !strings.Contains(text, `"not_applied"`) {
		t.Fatalf("unexpected result: %s", text)
	}
}
