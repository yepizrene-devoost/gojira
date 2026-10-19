package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegisterOneEntryAndRejectUnknown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a := adapter{}
	server := mcp.NewServer(&mcp.Implementation{Name: "table-test", Version: "1"}, nil)
	calls := 0
	// One entry alone supplies everything registration and dispatch need.
	a.register(server, []tool{{
		name: "extra", description: "Extra test tool", schema: objectSchema(map[string]any{}),
		handle: func(input) (any, error) { calls++; return map[string]any{"ok": true}, nil },
	}})
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ss.Close(); err != nil {
			t.Error(err)
		}
	}()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cs.Close(); err != nil {
			t.Error(err)
		}
	}()
	list, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil || len(list.Tools) != 1 || list.Tools[0].Name != "extra" {
		t.Fatalf("table: %v %v", list, err)
	}
	got, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "extra", Arguments: map[string]any{}})
	if err != nil || got.IsError || calls != 1 {
		t.Fatalf("extra: %v %v; calls %d", got, err, calls)
	}
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "unknown", Arguments: map[string]any{}}); err == nil {
		t.Fatal("unknown tool callable")
	}
	if calls != 1 {
		t.Fatal("unknown tool dispatched")
	}
}
