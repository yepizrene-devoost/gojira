package cmd

import (
	"io"
	"os"
	"strings"
	"testing"

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
