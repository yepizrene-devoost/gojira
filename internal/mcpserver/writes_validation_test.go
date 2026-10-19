package mcpserver

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWriteConfirmationConcurrent(t *testing.T) {
	store := newWriteConfirmations(time.Now)
	token, err := store.issue("ABC-1")
	if err != nil {
		t.Fatal(err)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if store.consume(token, "ABC-1") {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("redemptions=%d", wins.Load())
	}
}
func TestWriteValidation(t *testing.T) {
	for _, entry := range writeTools() {
		for _, raw := range []string{`null`, `[]`, `{"extra":true}`, `{"key":null}`, `{"key":12}`} {
			if _, message := validateWrite(entry, json.RawMessage(raw)); message == "" {
				t.Fatalf("%s accepted %s", entry.action, raw)
			}
		}
	}
	update := writeTools()[5]
	for _, raw := range []string{`{"key":"ABC-1","fields":{"description":"text"}}`, `{"key":"ABC-1","fields":{"summary":null}}`, `{"key":"ABC-1","fields":{}}`} {
		if _, message := validateWrite(update, json.RawMessage(raw)); message == "" {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestWriteCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// A nil writer must not be reached, and no preview token may be issued.
	a := adapter{}
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "gojira_delete", Arguments: json.RawMessage(`{"key":"ABC-1"}`)}}
	res, err := a.writeCall(writeTools()[6], nil)(ctx, req)
	if err != nil || !res.IsError {
		t.Fatalf("%v %v", res, err)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var value map[string]any
	_ = json.Unmarshal(raw, &value)
	if value["code"] != "cancelled" || value["mutationState"] != "not_applied" {
		t.Fatalf("%s", raw)
	}
}
