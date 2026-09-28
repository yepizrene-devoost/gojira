package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/spf13/cobra"
)

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestWriteJSONPropagatesEncodingError(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)

	err := writeJSON(cmd, make(chan int))
	if err == nil {
		t.Fatal("writeJSON() error = nil, want unsupported type error")
	}
	var unsupported *json.UnsupportedTypeError
	if !errors.As(err, &unsupported) {
		t.Fatalf("writeJSON() error = %T %v, want *json.UnsupportedTypeError", err, err)
	}
}

func TestWriteJSONPropagatesOutputWriterError(t *testing.T) {
	wantErr := errors.New("output unavailable")
	cmd := &cobra.Command{}
	cmd.SetOut(failingWriter{err: wantErr})

	err := writeJSON(cmd, map[string]string{"status": "ok"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("writeJSON() error = %v, want %v", err, wantErr)
	}
}

func assertSingleJSONDocument(t *testing.T, data []byte, target any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		t.Fatalf("first JSON decode failed: %v\noutput: %q", err, data)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("output contains more than one JSON document: error = %v, extra = %#v, output = %q", err, extra, data)
	}
}
