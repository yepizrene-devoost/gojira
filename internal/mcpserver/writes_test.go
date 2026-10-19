package mcpserver

import (
	"testing"
	"time"
)

func TestWriteConfirmation(t *testing.T) {
	now := time.Now()
	store := newWriteConfirmations(func() time.Time { return now })
	token, err := store.issue("ABC-1")
	if err != nil {
		t.Fatal(err)
	}
	if store.consume(token, "ABC-2") {
		t.Fatal("wrong key accepted")
	}
	token, err = store.issue("ABC-1")
	if err != nil {
		t.Fatal(err)
	}
	if !store.consume(token, "ABC-1") || store.consume(token, "ABC-1") {
		t.Fatal("token not single use")
	}
	token, err = store.issue("ABC-1")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if store.consume(token, "ABC-1") {
		t.Fatal("expired token accepted")
	}
}
