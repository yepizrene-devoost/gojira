package cmd

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadConfigInputPropagatesEOF(t *testing.T) {
	_, err := readConfigInput(bufio.NewReader(strings.NewReader("partial")), "domain")
	if !errors.Is(err, io.EOF) {
		t.Fatalf("readConfigInput() error = %v, want wrapped EOF", err)
	}
	if !strings.Contains(err.Error(), "reading domain") {
		t.Fatalf("readConfigInput() error = %v, want field context", err)
	}
}
