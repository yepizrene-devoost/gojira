package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWriteMissingWriterPreservesToken(t *testing.T) {
	store := newWriteConfirmations(time.Now)
	token, err := store.issue("ABC-1")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]string{"key": "ABC-1", "confirmationToken": token})
	res, err := (adapter{}).writeCall(writeTools()[6], store)(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: raw}})
	if err != nil || !res.IsError {
		t.Fatalf("%v %v", res, err)
	}
	if !store.consume(token, "ABC-1") {
		t.Fatal("missing writer burned confirmation token")
	}
}
func TestWriteDeleteConsentDescription(t *testing.T) {
	description := writeTools()[6].description
	for _, phrase := range []string{"second call", "single-call accidental delete", "not proof of human approval", "same agent"} {
		if !strings.Contains(description, phrase) {
			t.Errorf("description missing %q", phrase)
		}
	}
}
