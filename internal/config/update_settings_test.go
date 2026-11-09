package config

import (
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// Rule S1: update_check_interval parses as a duration and fails safe. An
// absent, zero, unparsable or negative value means "check on every start" —
// it must never degrade into "never check".
func TestUpdateCheckIntervalDurationFailsSafe(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "absent means every start", value: "", want: 0},
		{name: "explicit zero means every start", value: "0", want: 0},
		{name: "positive duration", value: "6h", want: 6 * time.Hour},
		{name: "surrounding whitespace", value: " 6h ", want: 6 * time.Hour},
		{name: "unparsable value behaves as every start", value: "daily", want: 0},
		{name: "negative value behaves as every start", value: "-6h", want: 0},
		{name: "unitless value behaves as every start", value: "6", want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{UpdateCheckInterval: tc.value}
			if got := cfg.UpdateCheckIntervalDuration(); got != tc.want {
				t.Fatalf("UpdateCheckIntervalDuration() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Rule S2: a successful check persists the tag and the response ETag under
// the documented YAML keys, and the two diagnostic keys keep working.
func TestRecordUpdateCheckResultPersistsETagAndDiagnostics(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	checkedAt := time.Date(2026, 11, 8, 9, 30, 0, 0, time.UTC)
	etag := `W/"abc123"`

	cfg := &Config{Domain: "example.atlassian.net", UpdateCheckInterval: "6h"}
	cfg.RecordUpdateCheckResult(checkedAt, "v0.7.0", etag)
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save returned unexpected error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if loaded.UpdateETag != etag {
		t.Fatalf("UpdateETag = %q, want %q", loaded.UpdateETag, etag)
	}
	if loaded.UpdateLatestVersion != "v0.7.0" {
		t.Fatalf("UpdateLatestVersion = %q, want v0.7.0", loaded.UpdateLatestVersion)
	}
	if loaded.UpdateLastCheck != "2026-11-08T09:30:00Z" {
		t.Fatalf("UpdateLastCheck = %q, want the recorded UTC timestamp", loaded.UpdateLastCheck)
	}
	if loaded.UpdateCheckInterval != "6h" {
		t.Fatalf("UpdateCheckInterval = %q, want 6h", loaded.UpdateCheckInterval)
	}
	if loaded.UpdateCheckIntervalDuration() != 6*time.Hour {
		t.Fatalf("UpdateCheckIntervalDuration() = %v, want 6h", loaded.UpdateCheckIntervalDuration())
	}
	if !loaded.UpdateCheckEnabled() {
		t.Fatal("UpdateCheckEnabled() = false, want the default true")
	}
	if loaded.UpdateCheckDue(checkedAt.Add(time.Hour), loaded.UpdateCheckIntervalDuration()) {
		t.Fatal("UpdateCheckDue() = true inside the configured interval, want throttled")
	}
	if !loaded.UpdateCheckDue(checkedAt.Add(7*time.Hour), loaded.UpdateCheckIntervalDuration()) {
		t.Fatal("UpdateCheckDue() = false after the configured interval, want due")
	}
}

// Rule S2: both new keys marshal under their documented YAML names.
func TestUpdateSettingsUseDocumentedYAMLKeys(t *testing.T) {
	cfg := &Config{UpdateETag: `"abc"`, UpdateCheckInterval: "6h"}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal returned unexpected error: %v", err)
	}
	documented := string(data)
	for _, key := range []string{"update_etag:", "update_check_interval:"} {
		if !strings.Contains(documented, key) {
			t.Fatalf("marshalled config %q does not contain key %q", documented, key)
		}
	}
}

// The pre-existing RecordUpdateCheck keeps its old behaviour: it records the
// timestamp and tag and never touches the stored ETag.
func TestRecordUpdateCheckKeepsETagUntouched(t *testing.T) {
	cfg := &Config{UpdateETag: `"old"`, UpdateLatestVersion: "v0.6.0"}
	cfg.RecordUpdateCheck(time.Date(2026, 11, 8, 9, 30, 0, 0, time.UTC), "v0.7.0")
	if cfg.UpdateETag != `"old"` {
		t.Fatalf("UpdateETag = %q, want the stored ETag unchanged", cfg.UpdateETag)
	}
	if cfg.UpdateLatestVersion != "v0.7.0" {
		t.Fatalf("UpdateLatestVersion = %q, want v0.7.0", cfg.UpdateLatestVersion)
	}
}
