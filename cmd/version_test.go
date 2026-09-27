package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"
)

func TestVersionTextReport(t *testing.T) {
	const testVersion = "v1.2.3"
	setTestVersion(t, testVersion)

	got := runVersionCommand(t, false)
	want := "gojira " + testVersion
	if revision := shortRevision(); revision != "" {
		want += " (" + revision
		if buildDirty() {
			want += "-dirty"
		}
		want += ")"
	}
	want += "\n"

	if got != want {
		t.Fatalf("version text output = %q, want %q", got, want)
	}
}

func TestVersionJSONReport(t *testing.T) {
	const testVersion = "v1.2.3"
	setTestVersion(t, testVersion)

	output := runVersionCommand(t, true)
	decoder := json.NewDecoder(bytes.NewBufferString(output))
	var report map[string]json.RawMessage
	if err := decoder.Decode(&report); err != nil {
		t.Fatalf("decode version JSON: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatalf("version JSON contains additional output: %v", err)
	}

	if len(report) != 3 {
		t.Fatalf("version JSON fields = %v, want exactly version, revision, dirty", report)
	}
	assertJSONField(t, report, "version", testVersion)
	assertJSONField(t, report, "revision", buildRevision())
	assertJSONField(t, report, "dirty", buildDirty())
}

func runVersionCommand(t *testing.T, jsonOutput bool) string {
	t.Helper()

	oldOut := versionCmd.OutOrStdout()
	var output bytes.Buffer
	versionCmd.SetOut(&output)
	if err := versionCmd.Flags().Set("json", fmt.Sprintf("%t", jsonOutput)); err != nil {
		t.Fatalf("set --json: %v", err)
	}
	t.Cleanup(func() {
		versionCmd.SetOut(oldOut)
		_ = versionCmd.Flags().Set("json", "false")
	})

	if err := versionCmd.RunE(versionCmd, nil); err != nil {
		t.Fatalf("run version command: %v", err)
	}
	return output.String()
}

func setTestVersion(t *testing.T, value string) {
	t.Helper()

	oldVersion := version
	oldRootVersion := rootCmd.Version
	oldVersionTemplate := rootCmd.VersionTemplate()
	SetVersion(value)
	t.Cleanup(func() {
		version = oldVersion
		rootCmd.Version = oldRootVersion
		rootCmd.SetVersionTemplate(oldVersionTemplate)
	})
}

func assertJSONField[T comparable](t *testing.T, report map[string]json.RawMessage, name string, want T) {
	t.Helper()

	raw, ok := report[name]
	if !ok {
		t.Fatalf("version JSON is missing %q", name)
	}
	var got T
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode version JSON field %q: %v", name, err)
	}
	if got != want {
		t.Fatalf("version JSON field %q = %v, want %v", name, got, want)
	}
}
