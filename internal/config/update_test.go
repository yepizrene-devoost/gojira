package config

import (
	"testing"
	"time"
)

func boolPointer(value bool) *bool { return &value }

func TestUpdateCheckIsEnabledUnlessDisabled(t *testing.T) {
	cases := []struct {
		name  string
		value *bool
		want  bool
	}{
		{name: "unset", value: nil, want: true},
		{name: "explicitly enabled", value: boolPointer(true), want: true},
		{name: "explicitly disabled", value: boolPointer(false), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{UpdateCheck: tc.value}
			if got := cfg.UpdateCheckEnabled(); got != tc.want {
				t.Fatalf("UpdateCheckEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUpdateCheckDueThrottlesByInterval(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		last string
		want bool
	}{
		{name: "never checked", last: "", want: true},
		{name: "checked an hour ago", last: now.Add(-time.Hour).Format(time.RFC3339), want: false},
		{name: "checked 25 hours ago", last: now.Add(-25 * time.Hour).Format(time.RFC3339), want: true},
		{name: "unparseable timestamp", last: "yesterday", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{UpdateLastCheck: tc.last}
			if got := cfg.UpdateCheckDue(now, UpdateCheckInterval); got != tc.want {
				t.Fatalf("UpdateCheckDue() = %v, want %v", got, tc.want)
			}
		})
	}
	if UpdateCheckInterval != 24*time.Hour {
		t.Fatalf("UpdateCheckInterval = %v, want 24h", UpdateCheckInterval)
	}
}

func TestRecordUpdateCheckSurvivesSaveAndLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	checkedAt := time.Date(2026, 11, 8, 9, 30, 0, 0, time.UTC)

	cfg := &Config{Domain: "example.atlassian.net"}
	cfg.RecordUpdateCheck(checkedAt, "v0.6.0")
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save returned unexpected error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if loaded.UpdateLastCheck != "2026-11-08T09:30:00Z" {
		t.Fatalf("UpdateLastCheck = %q, want the recorded UTC timestamp", loaded.UpdateLastCheck)
	}
	if loaded.UpdateLatestVersion != "v0.6.0" {
		t.Fatalf("UpdateLatestVersion = %q, want v0.6.0", loaded.UpdateLatestVersion)
	}
	if !loaded.UpdateCheckEnabled() {
		t.Fatal("UpdateCheckEnabled() = false after a recorded check, want the default true")
	}
	if loaded.UpdateCheckDue(checkedAt.Add(time.Hour), UpdateCheckInterval) {
		t.Fatal("UpdateCheckDue() = true an hour after a check, want it throttled")
	}
}
