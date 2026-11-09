package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// captureStdout runs run and returns what it wrote to os.Stdout. The config
// commands print through the standard library, so this is the only way to read
// their output without changing their contract.
func captureStdout(t *testing.T, run func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stdout = writer
	done := make(chan string, 1)
	go func() {
		captured, _ := io.ReadAll(reader)
		done <- string(captured)
	}()
	run()
	_ = writer.Close()
	os.Stdout = original
	return <-done
}

func TestConfigUpdateCheckPreferenceRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	client := jira.NewClient("http://unused.example", "email", "token")

	code, _, stderr := runTestCLI(t, client, "config", "set", "update_check", "false")
	if code != 0 {
		t.Fatalf("config set exit = %d, stderr = %q", code, stderr)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if loaded.UpdateCheckEnabled() {
		t.Fatal("UpdateCheckEnabled() = true after setting update_check false")
	}

	output := captureStdout(t, func() {
		if code, _, stderr := runTestCLI(t, client, "config", "get", "update_check"); code != 0 {
			t.Fatalf("config get exit = %d, stderr = %q", code, stderr)
		}
	})
	if output != "false\n" {
		t.Fatalf("config get update_check = %q, want false", output)
	}

	if code, _, _ := runTestCLI(t, client, "config", "set", "update_check", "true"); code != 0 {
		t.Fatal("config set update_check true was rejected")
	}
	output = captureStdout(t, func() {
		_, _, _ = runTestCLI(t, client, "config", "get", "update_check")
	})
	if output != "true\n" {
		t.Fatalf("config get update_check = %q, want true", output)
	}
}

func TestConfigUpdateCheckReportsTheDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	client := jira.NewClient("http://unused.example", "email", "token")

	output := captureStdout(t, func() {
		if code, _, stderr := runTestCLI(t, client, "config", "get", "update_check"); code != 0 {
			t.Fatalf("config get exit = %d, stderr = %q", code, stderr)
		}
	})
	if output != "true\n" {
		t.Fatalf("config get update_check = %q, want the default true", output)
	}
}

func TestConfigUpdateCheckRejectsInvalidValues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	client := jira.NewClient("http://unused.example", "email", "token")

	code, _, stderr := runTestCLI(t, client, "config", "set", "update_check", "maybe")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "update_check") {
		t.Fatalf("stderr = %q, want the offending key", stderr)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if !loaded.UpdateCheckEnabled() {
		t.Fatal("a rejected value changed the stored preference")
	}
}

// TestRecordUpdateCheckPreservesUnrelatedValues (rule e): recording a release
// check writes update_last_check and update_latest_version while preserving an
// unrelated pre-existing value (the regression the reviewer named).
func TestRecordUpdateCheckPreservesUnrelatedValues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	existing := &config.Config{
		Domain: "example.atlassian.net",
		Email:  "keep@example.com",
		Board:  "KEPT",
	}
	if err := existing.Save(); err != nil {
		t.Fatalf("Save returned unexpected error: %v", err)
	}

	recordUpdateCheck("v9.9.9")

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if loaded.UpdateLatestVersion != "v9.9.9" {
		t.Fatalf("UpdateLatestVersion = %q, want v9.9.9", loaded.UpdateLatestVersion)
	}
	if _, err := time.Parse(time.RFC3339, loaded.UpdateLastCheck); err != nil {
		t.Fatalf("UpdateLastCheck = %q, want an RFC3339 timestamp: %v", loaded.UpdateLastCheck, err)
	}
	if loaded.Email != "keep@example.com" {
		t.Fatalf("Email = %q, want the unrelated pre-existing value preserved", loaded.Email)
	}
	if loaded.Domain != "example.atlassian.net" {
		t.Fatalf("Domain = %q, want the unrelated pre-existing value preserved", loaded.Domain)
	}
	if loaded.Board != "KEPT" {
		t.Fatalf("Board = %q, want the unrelated pre-existing value preserved", loaded.Board)
	}
}

func TestConfigSetTokenRefusesToStoreWithUnparsableConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := home + "/.config/gojira/config.yaml"
	if err := os.MkdirAll(home+"/.config/gojira", 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	corrupt := "domain: [unclosed\n"
	if err := os.WriteFile(configPath, []byte(corrupt), 0o600); err != nil {
		t.Fatalf("write corrupt config: %v", err)
	}

	stored := 0
	original := saveToken
	saveToken = func(string) error { stored++; return nil }
	t.Cleanup(func() { saveToken = original })

	code, _, stderr := runTestCLI(t, jira.NewClient("http://unused.example", "email", "token"), "config", "set", "token", "value")
	if code != 1 {
		t.Fatalf("exit = %d, want 1 so a corrupt config is never reported as success", code)
	}
	if stored != 0 {
		t.Fatalf("token was stored %d times over a corrupt config, want 0", stored)
	}
	if !strings.Contains(stderr, "parsing") {
		t.Fatalf("stderr = %q, want the parse error", stderr)
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(content) != corrupt {
		t.Fatalf("config file = %q, want it untouched", content)
	}
}
